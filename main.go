package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ─── Config ─────────────────────────────────────────────────────────────────

func apiBase() string {
	if v := os.Getenv("API_BASE_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://platify.aukespot.com/"
}

func listenAddr() string {
	if v := os.Getenv("PORT"); v != "" {
		return ":" + v
	}
	return ":8080"
}

// ─── Data models ─────────────────────────────────────────────────────────────

type RecipeResponse struct {
	Recipe Recipe `json:"recipe"`
}

type Recipe struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Image       string    `json:"image"`
	Boards      []string  `json:"boards"`
	Tags        []Tag     `json:"tags"`
	Sections    []Section `json:"sections"`
}

type Tag struct {
	Name       string `json:"name"`
	LightColor string `json:"lightColor"`
	DarkColor  string `json:"darkColor"`
}

type Section struct {
	Title       string          `json:"title"`
	Ingredients []Ingredient    `json:"ingredients"`
	Items       []NutritionItem `json:"items"`
	Directions  []Direction     `json:"directions"`
	VideoLink   string          `json:"videoLink"`
	List        []ShoppingItem  `json:"list"`
}

type Ingredient struct {
	Name     string `json:"name"`
	Quantity string `json:"quantity"`
	Unit     string `json:"unit"`
}

type Direction struct {
	Heading string `json:"heading"`
	Text    string `json:"text"`
}

type NutritionItem struct {
	Name      string    `json:"name"`
	Thumbnail string    `json:"thumbnail"`
	Quantity  float64   `json:"quantity"`
	Unit      string    `json:"unit"`
	Nutrients Nutrients `json:"nutrients"`
}

type Nutrients struct {
	Energy   float64 `json:"energy"`
	Carbs    float64 `json:"carbs"`
	Proteins float64 `json:"proteins"`
	Fats     float64 `json:"fats"`
}

type ShoppingItem struct {
	Name string `json:"name"`
}

type Product struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Thumbnail string    `json:"thumbnail"`
	Unit      string    `json:"unit"`
	Nutrients Nutrients `json:"nutrients"`
}

// ─── Template helpers ────────────────────────────────────────────────────────

func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		// dict builds a map from key-value pairs for passing props to components.
		//   Usage: {{template "hero-image" (dict "Src" .Image "Alt" .Name)}}
		"dict": func(pairs ...any) map[string]any {
			m := make(map[string]any, len(pairs)/2)
			for i := 0; i < len(pairs)-1; i += 2 {
				m[pairs[i].(string)] = pairs[i+1]
			}
			return m
		},
		// formatQty joins a quantity and unit into a display string.
		"formatQty": func(qty, unit string) string {
			if qty == "" && unit == "" {
				return ""
			}
			if unit == "" {
				return qty
			}
			if qty == "" {
				return unit
			}
			return qty + " " + unit
		},
		// formatFloat renders a float without trailing zeros.
		"formatFloat": func(f float64) string {
			if f == float64(int(f)) {
				return fmt.Sprintf("%d", int(f))
			}
			return fmt.Sprintf("%.1f", f)
		},
		// inc adds 1 (used for 1-based step numbering).
		"inc": func(i int) int {
			return i + 1
		},
	}
}

// loadTemplates walks templates/ and parses every .html file into a single
// template set. Components use {{define "name"}} blocks; pages reference them
// via {{template "name" .}}.
func loadTemplates() *template.Template {
	tmpl := template.New("").Funcs(templateFuncMap())

	err := filepath.Walk("templates", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".html") {
			if _, err := tmpl.ParseFiles(path); err != nil {
				return fmt.Errorf("parsing %s: %w", path, err)
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	return tmpl
}

// ─── API client ──────────────────────────────────────────────────────────────

func fetchRecipe(id string) (*Recipe, error) {
	url := apiBase() + "/recipes/" + id
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var rr RecipeResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return nil, err
	}
	return &rr.Recipe, nil
}

func fetchProduct(id string) (*Product, error) {
	url := apiBase() + "/products/" + id
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var product Product
	if err := json.NewDecoder(resp.Body).Decode(&product); err != nil {
		return nil, err
	}
	return &product, nil
}

// ─── Handlers ────────────────────────────────────────────────────────────────

func handleHome(c *gin.Context) {
	c.HTML(http.StatusOK, "home", nil)
}

func handleRecipe(c *gin.Context) {
	id := c.Param("id")

	recipe, err := fetchRecipe(id)
	if err != nil {
		log.Printf("fetchRecipe(%q): %v", id, err)
		renderError(c, http.StatusInternalServerError,
			"Could not load recipe",
			"The recipe could not be loaded. Please try again later.")
		return
	}
	if recipe == nil {
		renderError(c, http.StatusNotFound,
			"Recipe not found",
			"This recipe does not exist or is no longer available.")
		return
	}

	c.HTML(http.StatusOK, "recipe", recipe)
}

func handleProduct(c *gin.Context) {
	id := c.Param("id")

	product, err := fetchProduct(id)
	if err != nil {
		log.Printf("fetchProduct(%q): %v", id, err)
		renderError(c, http.StatusInternalServerError,
			"Could not load product",
			"The product could not be loaded. Please try again later.")
		return
	}
	if product == nil {
		renderError(c, http.StatusNotFound,
			"Product not found",
			"This product does not exist or is no longer available.")
		return
	}

	c.HTML(http.StatusOK, "product", product)
}

// handleExampleRecipe renders the recipe page using testdata/example_recipe.json.
// Only registered in non-release mode (GIN_MODE != release).
func handleExampleRecipe(c *gin.Context) {
	data, err := os.ReadFile("testdata/example_recipe.json")
	if err != nil {
		renderError(c, http.StatusInternalServerError, "Example not found", "Could not read testdata/example_recipe.json.")
		return
	}
	var rr RecipeResponse
	if err := json.Unmarshal(data, &rr); err != nil {
		renderError(c, http.StatusInternalServerError, "Example invalid", "Could not parse testdata/example_recipe.json.")
		return
	}
	c.HTML(http.StatusOK, "recipe", rr.Recipe)
}

func handlePrivacyPolicy(c *gin.Context) {
	c.File("prev-website/privacy-policy/index.html")
}

func renderError(c *gin.Context, status int, title, message string) {
	c.HTML(status, "error", gin.H{
		"Title":   title,
		"Message": message,
	})
}

// ─── Main ────────────────────────────────────────────────────────────────────

func main() {
	router := gin.Default()
	router.SetHTMLTemplate(loadTemplates())
	router.Static("/static", "./static")

	router.GET("/", handleHome)
	router.GET("/recipes/:id", handleRecipe)
	router.GET("/products/:id", handleProduct)
	router.GET("/privacy-policy", handlePrivacyPolicy)

	// Dev-only: preview the recipe page with local example data.
	if gin.Mode() != gin.ReleaseMode {
		router.GET("/recipes/_example", handleExampleRecipe)
		log.Printf("Dev route registered: GET /recipes/_example")
	}

	addr := listenAddr()
	log.Printf("Platify website listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatal(err)
	}
}
