package client

import (
	"testing"
)

var (
	wifs = map[string]string{
		"memo":    "5KhRtadrBoa9MEke7AoH2z73qH6GJGKRzUnXVVkaprgcqKrXwwX",
		"active":  "5KjrKfLLRkDnY8cHYH2PkMofv6W4xwykatdqyUgQ7eCHDwkjAwf",
		"posting": "5K4YjdpHFUJpMoWV7u1KTnAaZy59N8oT4csQdwyyqhLqCyZZQ6U",
		"owner":   "5KVCK2NsPxQVHLcjD2FCNtGyp7agdrv8nn1WAn9HnDybNLBhsEm",
	}
)

func TestImportWif(t *testing.T) {
	client := &Client{
		Url:      "https://api.steemit.com",
		MaxRetry: 5,
	}

	for kType, wif := range wifs {
		err := client.ImportWif(kType, wif)
		if err != nil {
			t.Errorf("testImportWif error: %+v", err)
			return
		}
	}

	for kType, wif := range wifs {
		if client.Wifs[kType].ToWif() != wif {
			t.Errorf("TestImportWif unexpect wif, kType: %+v, expected: %+v, got: %+v", kType, wif, client.Wifs[kType].ToWif())
			return
		}
	}
}
