package cmd

import (
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
)

var (
	flagLogLimit int
	flagLogEvent string
)

type ActivityLogItem struct {
	ID        int    `json:"id"`
	Type      string `json:"type"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type DashboardMetricsResponse struct {
	TotalProjects  int `json:"total_projects"`
	TotalFiles     int `json:"total_files"`
	TotalDownloads int `json:"total_downloads"`
	TotalViews     int `json:"total_views"`
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View chronological telemetry activity logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		query := url.Values{}
		if flagLogLimit > 0 {
			query.Set("limit", strconv.Itoa(flagLogLimit))
		}
		if flagLogEvent != "" {
			query.Set("event", flagLogEvent)
		}

		raw, err := apiCli.Get("/api/v1/logs", query)
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		var items []ActivityLogItem
		if err := json.Unmarshal(raw, &items); err != nil {
			return err
		}

		if len(items) == 0 {
			printer.PrintInfo("No activity logs recorded.")
			return nil
		}

		headers := []string{"TIME", "TYPE", "STATUS", "SUMMARY"}
		var rows [][]string
		for _, item := range items {
			rows = append(rows, []string{
				item.CreatedAt,
				item.Type,
				item.Status,
				item.Summary,
			})
		}

		printer.Table(headers, rows)
		return nil
	},
}

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Display aggregated catalog and reader engagement metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		raw, err := apiCli.Get("/api/v1/dashboard/metrics", nil)
		if err != nil {
			return err
		}

		if printer.JSON {
			return printer.PrintRawJSON(raw)
		}

		var metrics DashboardMetricsResponse
		if err := json.Unmarshal(raw, &metrics); err != nil {
			return err
		}

		rows := [][]string{
			{"Total Book Projects", strconv.Itoa(metrics.TotalProjects)},
			{"Total Uploaded Files", strconv.Itoa(metrics.TotalFiles)},
			{"Total Reader Downloads", strconv.Itoa(metrics.TotalDownloads)},
			{"Total Landing Page Views", strconv.Itoa(metrics.TotalViews)},
		}

		printer.Table([]string{"Metric", "Total"}, rows)
		return nil
	},
}

func init() {
	logsCmd.Flags().IntVarP(&flagLogLimit, "limit", "n", 50, "Maximum number of log events to show")
	logsCmd.Flags().StringVar(&flagLogEvent, "event", "", "Filter logs by event type (e.g. signup, download)")

	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(metricsCmd)
}
