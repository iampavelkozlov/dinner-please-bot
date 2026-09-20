package recipes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "02-супы"), 0o750))
	require.NoError(t, os.Mkdir(filepath.Join(root, "01-завтраки"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "01-завтраки", "02.md"), []byte("*Второй рецепт*\n\nТекст"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "01-завтраки", "01.md"), []byte("*Первый рецепт*\n\nТекст"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "02-супы", "05.md"), []byte("*Суп*\n\nТекст"), 0o600))

	catalog, err := Load(root)
	require.NoError(t, err)
	require.Equal(t, []string{"Завтраки", "Супы"}, []string{catalog.Categories[0].Name, catalog.Categories[1].Name})
	require.Equal(t, []string{"Первый рецепт", "Второй рецепт"}, []string{
		catalog.Categories[0].Recipes[0].Title,
		catalog.Categories[0].Recipes[1].Title,
	})
}

func TestLoadErrors(t *testing.T) {
	tests := map[string]func(t *testing.T) string{
		"missing root": func(t *testing.T) string {
			return filepath.Join(t.TempDir(), "missing")
		},
		"no categories": func(t *testing.T) string {
			return t.TempDir()
		},
		"empty recipe": func(t *testing.T) string {
			root := t.TempDir()
			directory := filepath.Join(root, "01-завтраки")
			require.NoError(t, os.Mkdir(directory, 0o750))
			require.NoError(t, os.WriteFile(filepath.Join(directory, "01.md"), nil, 0o600))
			return root
		},
	}
	for name, setup := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Load(setup(t))
			require.Error(t, err)
		})
	}
}

func TestCategoryName(t *testing.T) {
	tests := map[string]string{
		"01-завтраки": "Завтраки",
		"супы":        "Супы",
		"":            "",
	}
	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			require.Equal(t, expected, categoryName(input))
		})
	}
}

func TestBundledCatalog(t *testing.T) {
	catalog, err := Load(filepath.Join("..", "..", "recipes"))
	require.NoError(t, err)
	require.Equal(t, []string{"Завтраки", "Супы", "Салаты", "Гарниры", "Мясо"}, []string{
		catalog.Categories[0].Name,
		catalog.Categories[1].Name,
		catalog.Categories[2].Name,
		catalog.Categories[3].Name,
		catalog.Categories[4].Name,
	})
	require.Equal(t, []int{8, 3, 9, 14, 8}, []int{
		len(catalog.Categories[0].Recipes),
		len(catalog.Categories[1].Recipes),
		len(catalog.Categories[2].Recipes),
		len(catalog.Categories[3].Recipes),
		len(catalog.Categories[4].Recipes),
	})
}
