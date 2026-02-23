package gog

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type OwnedGamesResponse struct {
	Owned []int `json:"owned"`
}

type Product struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

func (c *Client) GetOwnedGameIDs() ([]int, error) {
	resp, err := c.AuthGet("https://embed.gog.com/user/data/games")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get owned games (%d): %s", resp.StatusCode, body)
	}

	var result OwnedGamesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Owned, nil
}

func (c *Client) GetProducts(ids []int) ([]Product, error) {
	var all []Product
	// Batch in groups of 50
	for i := 0; i < len(ids); i += 50 {
		end := i + 50
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[i:end]

		strs := make([]string, len(batch))
		for j, id := range batch {
			strs[j] = fmt.Sprintf("%d", id)
		}
		url := "https://api.gog.com/products?ids=" + strings.Join(strs, ",")

		resp, err := c.AuthGet(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("failed to get products (%d): %s", resp.StatusCode, body)
		}

		var products []Product
		if err := json.NewDecoder(resp.Body).Decode(&products); err != nil {
			return nil, err
		}
		all = append(all, products...)
	}
	return all, nil
}
