package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Command-line flags.
var listen = flag.String("listen", ":8080", "address:port to listen on")
var label = flag.String("label", "", "graph label")
var min = flag.Float64("min", math.Inf(-1), "minimum accepted value")
var max = flag.Float64("max", math.Inf(1), "maximum accepted value")
var bucket = flag.Int("bucket", 1, "histogram bucket size")
var delay = flag.Int("delay", 5, "delay between updates, in seconds")
var wide = flag.Bool("wide", false, "use wide orientation")

// The type of the items to parse from stdin and count in the histogram.
type Countable float64

// Parses a countable value from a string, and returns a non-nil error if
// parsing fails.
func Parse(str string) (Countable, error) {
	d, err := strconv.ParseFloat(str, 64)
	return Countable(d), err
}

// Returns the bucket (as a string) of which the countable value should
// increment the count, given the bucket size.
func (d Countable) Bucket(size int) string {
	if d < 0 {
		d -= Countable(size)
	}
	return strconv.Itoa(int(d) / size * size)
}

// Collects and buckets values. Stats (min, max, total, etc.) are computed as
// countable values come in.
type Histogram struct {
	Values map[string]Countable

	Bucket int    // the histogram bucket size
	Label  string // the label of the histogram
	Wide   bool

	Min Countable // the minimum value encountered so far
	Max Countable // the maximum value encountered so far
	Sum Countable // the sum of values encountered so far

	Count    int // the number of values encountered so far
	Filtered int // the number of values filtered out so far
	Errors   int // the number of values skipped due to errors so far
}

// Returns a new histogram. The bucket size is used to count values that
// fall within a different size. The `label` and `wide` options control
// the display of the rendered graph.
func NewHistogram(bucket int, label string, wide bool) *Histogram {
	return &Histogram{
		Values: make(map[string]Countable, 1024),
		Bucket: bucket,
		Label:  label,
		Wide:   wide,
		Min:    Countable(math.Inf(1)),
		Max:    Countable(math.Inf(-1))}
}

// Adds a countable value, modifying the stats and counts accordingly.
func (hist *Histogram) Add(val Countable, err error) {
	if err != nil {
		hist.Errors += 1
		return
	} else if val < Countable(*min) || val > Countable(*max) {
		hist.Filtered += 1
		return
	}

	if val < hist.Min {
		hist.Min = val
	}
	if val > hist.Max {
		hist.Max = val
	}
	hist.Sum += val
	hist.Count += 1
	hist.Values[val.Bucket(hist.Bucket)] += 1
}

}

// Read and parse countable values from stdin, add them to a histogram and
// update stats.
func Read(hist *Histogram) {
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		hist.Add(Parse(strings.TrimSpace(line)))
	}
}

	}
}

func main() {
	flag.Parse()

	hist := NewHistogram(*bucket, *label, *wide)
	ticker := time.NewTicker(time.Duration(*delay) * time.Second)

	go Read(hist)

	indexpage := template.Must(template.ParseFiles("index.html"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		msg, err := json.Marshal(&hist)
		if err != nil {
			fmt.Println("FAIL", err)
			return
		}
		indexpage.Execute(w, string(msg))
	})

	http.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flusher", http.StatusInternalServerError)
			return
		}

		cn, ok := w.(http.CloseNotifier)
		if !ok {
			http.Error(w, "no close notifier", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		writeHist := func() bool {
			msg, err := json.Marshal(&hist)
			if err != nil {
				fmt.Println("FAIL", err)
				fmt.Fprint(w, "data: {\"type\": \"error\"}\n\n")
				return false
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			f.Flush()
			return true
		}

		lastcount := 0
		for {
			select {
			case _ = <-cn.CloseNotify():
				return

			case _ = <-ticker.C:
				if hist.Count <= lastcount {
					continue
				}
				lastcount = hist.Count

				if !writeHist() {
					return
				}
			}
		}
	})

	http.ListenAndServe(*listen, nil)
}
