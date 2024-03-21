package main

import (
	"compress/flate"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/a-h/templ"
	"github.com/andybalholm/brotli"
)

type PropsType struct {
	Tag        string
	Attributes templ.Attributes
	Children   []Component
	Text       string
}

type Component struct {
	WidgetId string
	Props    PropsType
}

type Cache struct {
	data map[string]interface{}
	mu   sync.RWMutex
}

func NewCache() *Cache {
	return &Cache{
		data: make(map[string]interface{}),
	}
}

func (c *Cache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.data[key]
	return value, ok
}

func attrToString(attributes templ.Attributes) string {
	data := []string{}

	for key, value := range attributes {
		data = append(data, fmt.Sprintf("%s=\"%s\"", key, value))
	}

	return strings.Join(data, " ")
}

func parseJsonFile(file string) ([]Component, error) {
	var obj []Component

	jsonFile, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	// Try unmarshaling as a single object
	if err := json.Unmarshal(byteValue, &obj); err != nil {
		// Try unmarshaling as an array
		var arr []Component
		if err := json.Unmarshal(byteValue, &arr); err != nil {
			return nil, err
		}
		return arr, nil
	}

	return obj, nil
}

func main() {
	cache := NewCache()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		pageId := r.URL.Path[1:]

		var page templ.Component
		cachedPage, ok := cache.Get(pageId)
		if ok {
			page = cachedPage.(templ.Component)
		} else {
			pageStyle := "flex flex-col h-screen overflow-y-hidden mx-auto items-center justify-center gap-4 w-screen bg-teal-700"
			widgets, err := parseJsonFile("data/" + pageId + ".widgets.json")

			if err != nil {
				fmt.Println(err)
				page = typography("Error while accessing url: "+pageId, "h1", templ.Attributes{})
			} else {
				page = layout(widgets, pageStyle)
			}

			cache.Set(pageId, page)
		}

		// Set the content encoding to brotli
		w.Header().Set("Content-Encoding", "br")

		// Create a brotli writer
		br := brotli.NewWriterOptions(w, brotli.WriterOptions{
			Quality: flate.BestSpeed, // Set compression quality
		})
		defer br.Close()

		// Render the page and write it to the brotli writer
		page.Render(r.Context(), br)
	})

	http.ListenAndServe(":3000", nil)
}
