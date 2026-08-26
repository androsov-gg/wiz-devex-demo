package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagProject          string
	flagRegion           string
	flagScenario         string
	flagIAPMember        string
	flagVerbose          bool
	flagEnableScheduler  string
	flagScheduleTimezone string
	flagScaleDownSched   string
	flagScaleUpSched     string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "deployer",
	Short: "Wiz DevEx Demo Deployment CLI for Google Cloud Argolis",
	Long: `A guided, interactive deployment tool for Wiz Customer Engineers to deploy
and manage Identity-Aware Proxy (IAP) secured code-server demo environments on Google Cloud Run.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagProject, "project", "p", "", "Google Cloud project ID (Argolis project)")
	rootCmd.PersistentFlags().StringVarP(&flagRegion, "region", "r", "europe-west1", "Google Cloud region (supported: europe-west1, us-east5, asia-southeast1)")
	rootCmd.PersistentFlags().StringVarP(&flagScenario, "scenario", "s", "", "Demo scenario name (from configurations/)")
	rootCmd.PersistentFlags().StringVarP(&flagIAPMember, "iap-member", "m", "", "Allowed IAP member (e.g. domain:yourname.altostrat.com)")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Enable verbose execution logging")
	rootCmd.PersistentFlags().StringVar(&flagEnableScheduler, "enable-scheduler", "", "Enable Cloud Scheduler off-hours scaling (true/false)")
	rootCmd.PersistentFlags().StringVar(&flagScheduleTimezone, "schedule-timezone", "", "Timezone for Cloud Scheduler scaling jobs (e.g. America/New_York, UTC)")
	rootCmd.PersistentFlags().StringVar(&flagScaleDownSched, "scale-down-schedule", "", "Schedule for scaling down to 0 instances (e.g. 19:00, 0 19 * * 1-5)")
	rootCmd.PersistentFlags().StringVar(&flagScaleUpSched, "scale-up-schedule", "", "Schedule for scaling up to 1 instance (e.g. 08:00, 0 8 * * 1-5)")
}
