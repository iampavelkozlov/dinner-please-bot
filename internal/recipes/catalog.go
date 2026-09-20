package recipes

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Catalog struct {
	Categories []Category
}

type Category struct {
	Name    string
	Recipes []Recipe
}

type Recipe struct {
	Title    string
	Markdown string
}

func Load(root string) (*Catalog, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read recipes directory: %w", err)
	}

	directories := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			directories = append(directories, entry)
		}
	}
	slices.SortFunc(directories, func(a, b os.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})

	catalog := &Catalog{Categories: make([]Category, 0, len(directories))}
	for _, directory := range directories {
		category, err := loadCategory(filepath.Join(root, directory.Name()), categoryName(directory.Name()))
		if err != nil {
			return nil, err
		}
		if len(category.Recipes) > 0 {
			catalog.Categories = append(catalog.Categories, category)
		}
	}
	if len(catalog.Categories) == 0 {
		return nil, errorsNoRecipes(root)
	}

	return catalog, nil
}

func loadCategory(path, name string) (Category, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return Category{}, fmt.Errorf("read category %q: %w", name, err)
	}

	files := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			files = append(files, entry)
		}
	}
	slices.SortFunc(files, func(a, b os.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})

	category := Category{Name: name, Recipes: make([]Recipe, 0, len(files))}
	for _, file := range files {
		content, err := os.ReadFile(filepath.Join(path, file.Name()))
		if err != nil {
			return Category{}, fmt.Errorf("read recipe %q: %w", file.Name(), err)
		}
		markdown := strings.TrimSpace(string(content))
		if markdown == "" {
			return Category{}, fmt.Errorf("recipe %q is empty", file.Name())
		}
		category.Recipes = append(category.Recipes, Recipe{
			Title:    recipeTitle(markdown),
			Markdown: markdown,
		})
	}

	return category, nil
}

func categoryName(directory string) string {
	if _, name, ok := strings.Cut(directory, "-"); ok {
		directory = name
	}
	directory = strings.ReplaceAll(directory, "-", " ")
	first, size := utf8.DecodeRuneInString(directory)
	if first == utf8.RuneError && size == 0 {
		return directory
	}
	return string(unicode.ToUpper(first)) + directory[size:]
}

func recipeTitle(markdown string) string {
	first, _, _ := strings.Cut(markdown, "\n")
	return strings.Trim(strings.TrimSpace(first), "*# ")
}

func errorsNoRecipes(root string) error {
	return fmt.Errorf("no recipe categories found in %q", root)
}
