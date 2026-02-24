package cmd

import (
	"fmt"
	"html"
	"regexp"
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

		fmt.Printf("\nFetching details for %s...\n\n", selected.Title)
		details, err := client.GetProductDetails(selected.ID)
		if err != nil {
			return err
		}

		fmt.Printf("  %s\n", details.Title)
		fmt.Printf("  %s\n\n", strings.Repeat("─", len(details.Title)))

		if details.ReleaseDate != "" {
			fmt.Printf("  Release Date:  %s\n", details.ReleaseDate)
		}

		var platforms []string
		if details.ContentSystemCompatibility.Windows {
			platforms = append(platforms, "Windows")
		}
		if details.ContentSystemCompatibility.OSX {
			platforms = append(platforms, "macOS")
		}
		if details.ContentSystemCompatibility.Linux {
			platforms = append(platforms, "Linux")
		}
		if len(platforms) > 0 {
			fmt.Printf("  Platforms:     %s\n", strings.Join(platforms, ", "))
		}

		if len(details.Languages) > 0 {
			langs := make([]string, 0, len(details.Languages))
			for _, name := range details.Languages {
				langs = append(langs, name)
			}
			sort.Strings(langs)
			fmt.Printf("  Languages:     %s\n", strings.Join(langs, ", "))
		}

		if details.Links.ProductCard != "" {
			fmt.Printf("  Store Page:    https://www.gog.com%s\n", details.Links.ProductCard)
		}

		if details.Description != nil && details.Description.Lead != "" {
			fmt.Printf("\n  %s\n", stripHTML(details.Description.Lead))
		}

		fmt.Println()
		return nil
	},
}

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, "")
	return html.UnescapeString(s)
}

func init() {
	rootCmd.AddCommand(listCmd)
}
