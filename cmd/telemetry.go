package cmd

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// maxLogsPerPage is the largest per_page value GET /api/v1/logs accepts.
const maxLogsPerPage = 100

// ActivityLogItem is one telemetry event; signup events carry no filename.
type ActivityLogItem struct {
	ID             int    `json:"id"`
	Type           string `json:"type"`
	OccurredAt     string `json:"occurred_at"`
	ReaderEmail    string `json:"reader_email"`
	BookTitle      string `json:"book_title"`
	Filename       string `json:"filename"`
	SignupLinkSlug string `json:"signup_link_slug"`
}

// ActivityLogPage is the paginator GET /api/v1/logs wraps its events in.
type ActivityLogPage struct {
	Data []ActivityLogItem `json:"data"`
}

type DashboardMetricsResponse struct {
	TotalProjects  int `json:"total_projects"`
	TotalFiles     int `json:"total_files"`
	TotalDownloads int `json:"total_downloads"`
	TotalViews     int `json:"total_views"`
}

func logsCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View chronological telemetry activity logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			query := url.Values{}
			limit, _ := cmd.Flags().GetInt("limit")
			event, _ := cmd.Flags().GetString("event")
			if limit > 0 {
				query.Set("per_page", strconv.Itoa(min(limit, maxLogsPerPage)))
			}

			raw, err := a.apiCli.Get("/api/v1/logs", query)
			if err != nil {
				return err
			}

			if a.printer.JSON {
				return a.printer.PrintRawJSON(raw)
			}

			var page ActivityLogPage
			if err := json.Unmarshal(raw, &page); err != nil {
				return err
			}

			items := logsOfType(page.Data, event)
			if len(items) == 0 {
				a.printer.PrintInfo("No activity logs recorded.")
				return nil
			}

			headers := []string{"TIME", "TYPE", "READER", "BOOK", "FILE"}
			var rows [][]string
			for _, item := range items {
				rows = append(rows, []string{
					item.OccurredAt,
					item.Type,
					item.ReaderEmail,
					item.BookTitle,
					item.Filename,
				})
			}

			a.printer.Table(headers, rows)
			return nil
		},
	}
	cmd.Flags().IntP("limit", "n", 50, "Maximum number of log events to show (capped at 100)")
	cmd.Flags().String("event", "", "Filter table rows by event type (e.g. signup, download); --json stays unfiltered")
	return cmd
}

// logsOfType keeps the events matching event, which the API has no filter for.
func logsOfType(items []ActivityLogItem, event string) []ActivityLogItem {
	if event == "" {
		return items
	}
	var matched []ActivityLogItem
	for _, item := range items {
		if strings.EqualFold(item.Type, event) {
			matched = append(matched, item)
		}
	}
	return matched
}

func metricsCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "metrics",
		Short: "Display aggregated catalog and reader engagement metrics",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := a.apiCli.Get("/api/v1/dashboard/metrics", nil)
			if err != nil {
				return err
			}

			if a.printer.JSON {
				return a.printer.PrintRawJSON(raw)
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

			a.printer.Table([]string{"Metric", "Total"}, rows)
			return nil
		},
	}
}
