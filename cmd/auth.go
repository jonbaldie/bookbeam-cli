package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/config"
	"github.com/spf13/cobra"
)

var (
	flagDirectToken string
)

type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type DeviceTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	TokenID     int    `json:"token_id"`
	TeamID      int    `json:"team_id"`
}

type UserProfileResponse struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	CurrentTeam *struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"current_team,omitempty"`
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication and tokens",
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with BookBeam via browser or direct token",
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagDirectToken != "" {
			cfg.Token = flagDirectToken
			if err := config.Save(cfg, ""); err != nil {
				return fmt.Errorf("failed to save token to config: %w", err)
			}
			printer.PrintInfo("✓ Authentication token saved successfully.")
			return nil
		}

		printer.PrintInfo(fmt.Sprintf("Initiating device login with %s...", cfg.Host))

		codePayload := map[string]string{
			"client_id": "bookbeam-cli",
		}
		rawResp, err := apiCli.Post("/api/v1/oauth/device/code", codePayload)
		if err != nil {
			return fmt.Errorf("failed to request device authorization code: %w", err)
		}

		var deviceResp DeviceCodeResponse
		if err := json.Unmarshal(rawResp, &deviceResp); err != nil {
			return fmt.Errorf("invalid server response: %w", err)
		}

		printer.PrintInfo("")
		printer.PrintInfo(fmt.Sprintf("! Your one-time verification code is: %s", deviceResp.UserCode))
		printer.PrintInfo(fmt.Sprintf("- Open verification page: %s", deviceResp.VerificationURIComplete))
		printer.PrintInfo("")

		openBrowser(deviceResp.VerificationURIComplete)

		pollInterval := deviceResp.Interval
		if pollInterval < 1 {
			pollInterval = 5
		}

		deadline := time.Now().Add(time.Duration(deviceResp.ExpiresIn) * time.Second)
		tokenPayload := map[string]string{
			"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
			"device_code": deviceResp.DeviceCode,
			"client_id":   "bookbeam-cli",
		}

		printer.PrintInfo("Waiting for authorization in browser...")

		for {
			if time.Now().After(deadline) {
				return fmt.Errorf("device authorization timed out; please run 'bookbeam auth login' again")
			}

			time.Sleep(time.Duration(pollInterval) * time.Second)

			tokenRaw, err := apiCli.Post("/api/v1/oauth/device/token", tokenPayload)
			if err == nil {
				var tokenResp DeviceTokenResponse
				if err := json.Unmarshal(tokenRaw, &tokenResp); err != nil {
					return fmt.Errorf("failed to parse token response: %w", err)
				}

				cfg.Token = tokenResp.AccessToken
				if err := config.Save(cfg, ""); err != nil {
					return fmt.Errorf("failed to save token to configuration: %w", err)
				}

				printer.PrintInfo("")
				printer.PrintInfo("✓ Successfully authenticated! Logged in to BookBeam.")
				return nil
			}

			apiErr, ok := err.(*client.APIError)
			if !ok {
				return err
			}

			if apiErr.ErrorType == "authorization_pending" {
				continue
			}

			if apiErr.ErrorType == "slow_down" {
				pollInterval += 5
				continue
			}

			return fmt.Errorf("authorization failed: %s", apiErr.ErrorDescription)
		}
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of BookBeam and remove saved credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg.Token = ""
		if err := config.Save(cfg, ""); err != nil {
			return fmt.Errorf("failed to update config file: %w", err)
		}
		printer.PrintInfo("✓ Logged out successfully.")
		return nil
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Display the currently authenticated user and team",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.Token == "" {
			printer.PrintError("Error: Not authenticated. Run 'bookbeam auth login' to authenticate.")
			os.Exit(4)
		}

		rawResp, err := apiCli.Get("/api/v1/user", nil)
		if err != nil {
			return fmt.Errorf("failed to get user details: %w", err)
		}

		if printer.JSON {
			return printer.PrintRawJSON(rawResp)
		}

		var profile UserProfileResponse
		if err := json.Unmarshal(rawResp, &profile); err != nil {
			return fmt.Errorf("failed to parse profile response: %w", err)
		}

		teamName := "Personal"
		if profile.CurrentTeam != nil && profile.CurrentTeam.Name != "" {
			teamName = profile.CurrentTeam.Name
		}

		rows := [][]string{
			{"Name", profile.Name},
			{"Email", profile.Email},
			{"Active Team", teamName},
			{"API Host", cfg.Host},
		}

		printer.Table([]string{"Property", "Value"}, rows)
		return nil
	},
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func init() {
	loginCmd.Flags().StringVar(&flagDirectToken, "token", "", "Authenticate directly with an API personal access token")

	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(whoamiCmd)

	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(whoamiCmd)
}
