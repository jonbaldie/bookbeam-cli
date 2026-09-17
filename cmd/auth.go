package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/jonbaldie/bookbeam-cli/pkg/client"
	"github.com/jonbaldie/bookbeam-cli/pkg/config"
	"github.com/spf13/cobra"
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

func loginCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with BookBeam via browser or direct token",
		RunE: func(cmd *cobra.Command, args []string) error {
			directToken, _ := cmd.Flags().GetString("token")
			if directToken != "" {
				if err := saveToken(a, directToken); err != nil {
					return err
				}
				a.printer.PrintInfo("✓ Authentication token saved successfully.")
				return nil
			}

			a.printer.PrintInfo(fmt.Sprintf("Initiating device login with %s...", a.cfg.Host))

			codePayload := map[string]string{
				"client_id": "bookbeam-cli",
			}
			rawResp, err := a.apiCli.Post("/api/v1/oauth/device/code", codePayload)
			if err != nil {
				return fmt.Errorf("failed to request device authorization code: %w", err)
			}

			var deviceResp DeviceCodeResponse
			if err := json.Unmarshal(rawResp, &deviceResp); err != nil {
				return fmt.Errorf("invalid server response: %w", err)
			}

			a.printer.PrintInfo("")
			a.printer.PrintInfo(fmt.Sprintf("! Your one-time verification code is: %s", deviceResp.UserCode))
			a.printer.PrintInfo(fmt.Sprintf("- Open verification page: %s", deviceResp.VerificationURIComplete))
			a.printer.PrintInfo("")

			a.openURL(deviceResp.VerificationURIComplete)

			a.printer.PrintInfo("Waiting for authorization in browser...")

			token, err := pollDeviceToken(a.apiCli, deviceResp, a.sleep)
			if err != nil {
				return err
			}
			if err := saveToken(a, token); err != nil {
				return err
			}

			a.printer.PrintInfo("")
			a.printer.PrintInfo("✓ Successfully authenticated! Logged in to BookBeam.")
			return nil
		},
	}
	cmd.Flags().String("token", "", "Authenticate directly with an API personal access token")
	return cmd
}

func logoutCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Log out of BookBeam and remove saved credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			a.cfg.Token = ""
			if err := config.Save(a.cfg, ""); err != nil {
				return fmt.Errorf("failed to update config file: %w", err)
			}
			a.printer.PrintInfo("✓ Logged out successfully.")
			return nil
		},
	}
}

func whoamiCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Display the currently authenticated user and team",
		RunE: func(cmd *cobra.Command, args []string) error {
			if a.cfg.Token == "" {
				cmd.SilenceUsage = true
				return errors.New("Not authenticated. Run 'bookbeam auth login' to authenticate.")
			}

			rawResp, err := a.apiCli.Get("/api/v1/user", nil)
			if err != nil {
				return fmt.Errorf("failed to get user details: %w", err)
			}

			if a.printer.JSON {
				return a.printer.PrintRawJSON(rawResp)
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
				{"API Host", a.cfg.Host},
			}

			a.printer.Table([]string{"Property", "Value"}, rows)
			return nil
		},
	}
}

func openBrowser(url string) {
	_ = browserCommand(runtime.GOOS, url).Start()
}

// browserCommand returns the platform's command for opening url in the default browser.
func browserCommand(goos, url string) *exec.Cmd {
	switch goos {
	case "darwin":
		return exec.Command("open", url)
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}
	return exec.Command("xdg-open", url)
}

func authCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication and tokens",
	}
	cmd.AddCommand(loginCmd(a), logoutCmd(a), whoamiCmd(a))
	return cmd
}

func saveToken(a *app, token string) error {
	a.cfg.Token = token
	if err := config.Save(a.cfg, ""); err != nil {
		return fmt.Errorf("failed to save token to config: %w", err)
	}
	return nil
}

// pollDeviceToken polls the token endpoint until the user approves the device code, it expires, or the server refuses it.
func pollDeviceToken(apiCli *client.Client, deviceResp DeviceCodeResponse, sleep func(time.Duration)) (string, error) {
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

	for time.Now().Before(deadline) {
		sleep(time.Duration(pollInterval) * time.Second)

		tokenRaw, err := apiCli.Post("/api/v1/oauth/device/token", tokenPayload)
		if err == nil {
			return parseDeviceToken(tokenRaw)
		}

		extra, err := pollBackoff(err)
		if err != nil {
			return "", err
		}
		pollInterval += extra
	}
	return "", fmt.Errorf("device authorization timed out; please run 'bookbeam auth login' again")
}

func parseDeviceToken(raw []byte) (string, error) {
	var tokenResp DeviceTokenResponse
	if err := json.Unmarshal(raw, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}
	return tokenResp.AccessToken, nil
}

// pollBackoff returns the extra seconds to wait before polling again, or the error that ends polling.
func pollBackoff(err error) (int, error) {
	apiErr, ok := err.(*client.APIError)
	if !ok {
		return 0, err
	}
	switch apiErr.ErrorType {
	case "authorization_pending":
		return 0, nil
	case "slow_down":
		return 5, nil
	}
	return 0, fmt.Errorf("authorization failed: %s", apiErr.ErrorDescription)
}
