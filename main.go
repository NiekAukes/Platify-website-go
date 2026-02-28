package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

// ─── Config ─────────────────────────────────────────────────────────────────

func apiBase() string {
	if v := os.Getenv("API_BASE_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://api.platify.eu"
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

// ─── Templates ───────────────────────────────────────────────────────────────

var templates *template.Template

func loadTemplates() {
	funcMap := template.FuncMap{
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
		"formatFloat": func(f float64) string {
			if f == float64(int(f)) {
				return fmt.Sprintf("%d", int(f))
			}
			return fmt.Sprintf("%.1f", f)
		},
		"inc": func(i int) int {
			return i + 1
		},
	}
	templates = template.Must(template.New("").Funcs(funcMap).ParseGlob("templates/*.html"))
}

// ─── API helpers ─────────────────────────────────────────────────────────────

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

func fetchProducts() ([]Product, error) {
	url := apiBase() + "/products"
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
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	var products []Product
	if err := json.NewDecoder(resp.Body).Decode(&products); err != nil {
		return nil, err
	}
	return products, nil
}

// ─── Handlers ────────────────────────────────────────────────────────────────

func handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if err := templates.ExecuteTemplate(w, "home.html", nil); err != nil {
		log.Printf("template error: %v", err)
	}
}

func handleRecipe(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/recipes/")
	id = path.Base(id)
	if id == "" || id == "." || id == "/" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	recipe, err := fetchRecipe(id)
	if err != nil {
		log.Printf("fetchRecipe(%q): %v", id, err)
		renderError(w, http.StatusInternalServerError, "Could not load recipe", "The recipe could not be loaded. Please try again later.")
		return
	}
	if recipe == nil {
		renderError(w, http.StatusNotFound, "Recipe not found", "This recipe does not exist or is no longer available.")
		return
	}

	if err := templates.ExecuteTemplate(w, "recipe.html", recipe); err != nil {
		log.Printf("template error: %v", err)
	}
}

func handleProducts(w http.ResponseWriter, r *http.Request) {
	products, err := fetchProducts()
	if err != nil {
		log.Printf("fetchProducts: %v", err)
		renderError(w, http.StatusInternalServerError, "Could not load products", "The product list could not be loaded. Please try again later.")
		return
	}

	if err := templates.ExecuteTemplate(w, "products.html", products); err != nil {
		log.Printf("template error: %v", err)
	}
}

func handlePrivacyPolicy(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "prev-website/privacy-policy/index.html")
}

func renderError(w http.ResponseWriter, status int, title, message string) {
	w.WriteHeader(status)
	data := struct {
		Title   string
		Message string
	}{title, message}
	if err := templates.ExecuteTemplate(w, "error.html", data); err != nil {
		log.Printf("error template: %v", err)
		http.Error(w, message, status)
	}
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	loadTemplates()

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleHome)
	mux.HandleFunc("/recipes/", handleRecipe)
	mux.HandleFunc("/products", handleProducts)
	mux.HandleFunc("/privacy-policy", handlePrivacyPolicy)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	addr := listenAddr()
	log.Printf("Platify website listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
