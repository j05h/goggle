package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/josh/goggle/pkg/gog"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List your GOG library",
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
		fmt.Printf("Found %d games. Fetching details...\n", len(ids))

		products, err := client.GetProducts(ids)
		if err != nil {
			return err
		}

		sort.Slice(products, func(i, j int) bool {
			return products[i].Title < products[j].Title
		})

		templates := &promptui.SelectTemplates{
			Label:    "{{ . }}",
			Active:   "\u25b8 {{ .Title | cyan }}",
			Inactive: "  {{ .Title }}",
			Selected: "\u2714 {{ .Title | green }}",
		}

		searcher := func(input string, index int) bool {
			product := products[index]
			return strings.Contains(
				strings.ToLower(product.Title),
				strings.ToLower(input),
			)
		}

		prompt := promptui.Select{
			Label:     "Your GOG Library",
			Items:     products,
			Templates: templates,
			Size:      20,
			Searcher:  searcher,
		}

		idx, _, err := prompt.Run()
		if err != nil {
			return err
		}

		selected := products[idx]
		fmt.Printf("Selected: %s (ID: %d)\n", selected.Title, selected.ID)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
