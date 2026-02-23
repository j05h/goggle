package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/josh/goggle/pkg/gog"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var downloadOS string

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download a game from your GOG library",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := gog.NewClient()
		if err != nil {
			return err
		}

		fmt.Println("Fetching library...")
		ids, err := client.GetOwnedGameIDs()
		if err != nil {
			return err
		}

		products, err := client.GetProducts(ids)
		if err != nil {
			return err
		}

		sort.Slice(products, func(i, j int) bool {
			return products[i].Title < products[j].Title
		})

		// Pick a game
		templates := &promptui.SelectTemplates{
			Active:   "\u25b8 {{ .Title | cyan }}",
			Inactive: "  {{ .Title }}",
			Selected: "\u2714 {{ .Title | green }}",
		}
		searcher := func(input string, index int) bool {
			return strings.Contains(
				strings.ToLower(products[index].Title),
				strings.ToLower(input),
			)
		}
		gamePrompt := promptui.Select{
			Label:     "Select a game to download",
			Items:     products,
			Templates: templates,
			Size:      20,
			Searcher:  searcher,
		}
		idx, _, err := gamePrompt.Run()
		if err != nil {
			return err
		}
		selected := products[idx]

		fmt.Printf("Fetching details for %s...\n", selected.Title)
		details, err := client.GetGameDetails(selected.ID)
		if err != nil {
			return err
		}

		installers, err := gog.ParseInstallers(details)
		if err != nil {
			return err
		}

		targetOS := downloadOS
		if targetOS == "" {
			targetOS = gog.DetectOS()
		}

		filtered := gog.FilterInstallersByOS(installers, targetOS)
		if len(filtered) == 0 {
			return fmt.Errorf("no %s installers found for %s", targetOS, selected.Title)
		}

		// Pick installer if multiple
		var chosen gog.Installer
		if len(filtered) == 1 {
			chosen = filtered[0]
		} else {
			instTemplates := &promptui.SelectTemplates{
				Active:   "\u25b8 {{ .Name | cyan }} ({{ .Size }}, {{ .Language }})",
				Inactive: "  {{ .Name }} ({{ .Size }}, {{ .Language }})",
				Selected: "\u2714 {{ .Name | green }}",
			}
			instPrompt := promptui.Select{
				Label:     "Select installer",
				Items:     filtered,
				Templates: instTemplates,
			}
			instIdx, _, err := instPrompt.Run()
			if err != nil {
				return err
			}
			chosen = filtered[instIdx]
		}

		fmt.Printf("Resolving download URL for %s...\n", chosen.Name)
		dlURL, err := client.ResolveDownloadURL(chosen.ManualURL)
		if err != nil {
			return err
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		destDir := filepath.Join(home, "GOG Games", selected.Title)

		fmt.Printf("Downloading to %s...\n", destDir)
		path, err := client.DownloadFile(dlURL, destDir)
		if err != nil {
			return err
		}

		fmt.Printf("Done! Saved to %s\n", path)
		return nil
	},
}

func init() {
	downloadCmd.Flags().StringVar(&downloadOS, "os", "", "Target OS (windows, mac, linux). Defaults to current OS.")
	rootCmd.AddCommand(downloadCmd)
}
