package memory

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math"
	"regexp"
	"strings"
	"time"
)

// Record representa una entrada atómica de memoria de contexto o decisión.
type Record struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id,omitempty"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Category  string    `json:"category,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SearchResult representa un resultado de búsqueda ordenado por relevancia BM25.
type SearchResult struct {
	Record Record  `json:"record"`
	Score  float64 `json:"score"`
}

// GenerateID genera un identificador aleatorio de 12 caracteres.
func GenerateID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))[:12]
	}
	return hex.EncodeToString(b)
}

// Validate verifica las reglas obligatorias del registro de memoria.
func (r *Record) Validate() error {
	if strings.TrimSpace(r.Title) == "" {
		return errors.New("el título de la memoria es obligatorio")
	}
	if strings.TrimSpace(r.Content) == "" {
		return errors.New("el contenido de la memoria es obligatorio")
	}
	return nil
}

var wordRegexp = regexp.MustCompile(`[\p{L}\p{N}]+`)

// Tokenize convierte un texto en un slice de términos normalizados en minúsculas.
func Tokenize(text string) []string {
	matches := wordRegexp.FindAllString(strings.ToLower(text), -1)
	return matches
}

// BM25Search ordena un conjunto de registros según la consulta usando el algoritmo Okapi BM25.
func BM25Search(records []Record, query string, limit int) []SearchResult {
	queryTokens := Tokenize(query)
	if len(queryTokens) == 0 || len(records) == 0 {
		return nil
	}

	totalDocs := float64(len(records))
	docLengths := make([]float64, len(records))
	docTermFreqs := make([]map[string]float64, len(records))
	docTermCount := make(map[string]int)

	var totalLen float64
	for i, r := range records {
		tf := make(map[string]float64)

		// Título con multiplicador x2.0
		for _, w := range Tokenize(r.Title) {
			tf[w] += 2.0
		}
		// Etiquetas con multiplicador x1.5
		for _, tag := range r.Tags {
			for _, w := range Tokenize(tag) {
				tf[w] += 1.5
			}
		}
		// Contenido con peso base x1.0
		for _, w := range Tokenize(r.Content) {
			tf[w] += 1.0
		}
		// Categoría
		for _, w := range Tokenize(r.Category) {
			tf[w] += 1.0
		}

		docTermFreqs[i] = tf
		length := float64(len(tf))
		docLengths[i] = length
		totalLen += length

		// Conteo de documentos que contienen cada término
		seen := make(map[string]bool)
		for term := range tf {
			if !seen[term] {
				seen[term] = true
				docTermCount[term]++
			}
		}
	}

	avgDL := totalLen / totalDocs
	if avgDL == 0 {
		avgDL = 1.0
	}

	k1 := 1.2
	b := 0.75

	var results []SearchResult
	for i, r := range records {
		score := 0.0
		tf := docTermFreqs[i]
		docLen := docLengths[i]

		for _, q := range queryTokens {
			freq, exists := tf[q]
			if !exists || freq == 0 {
				continue
			}

			n := float64(docTermCount[q])
			idf := math.Log((totalDocs - n + 0.5) / (n + 0.5) + 1.0)
			if idf < 0 {
				idf = 0.01
			}

			num := freq * (k1 + 1.0)
			den := freq + k1*(1.0-b+b*(docLen/avgDL))
			score += idf * (num / den)
		}

		if score > 0 {
			results = append(results, SearchResult{
				Record: r,
				Score:  score,
			})
		}
	}

	// Ordenar resultados por puntaje descendente
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}
