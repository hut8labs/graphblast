package main

import (
	"flag"
	"github.com/hut8labs/graphblast"
	"math"
	"net/http"
	"os"
	"time"
)

// Command-line flags.
var listen = flag.String("listen", ":8080", "address:port to listen on")
var verbose = flag.Bool("verbose", false, "be more verbose")
var label = flag.String("label", "", "graph label")
var min = flag.Float64("min", math.Inf(-1), "minimum accepted value")
var max = flag.Float64("max", math.Inf(1), "maximum accepted value")
var bucket = flag.Int("bucket", 1, "histogram bucket size")
var delay = flag.Int("delay", 5, "delay between updates, in seconds")
var wide = flag.Bool("wide", false, "use wide orientation")
var width = flag.Int("width", 500, "width of the graph, in pixels")
var height = flag.Int("height", 500, "height of the graph, in pixels")
var colors = flag.String("colors", "", "comma-separated: bg, fg, bar color")
var fontSize = flag.String("font-size", "", "font size (CSS)")
var window = flag.Int("window", 1000, "data window size")

func buildGraph(arg string) graphblast.Graph {
	allowed := graphblast.Range{
		Min: graphblast.Countable(*min),
		Max: graphblast.Countable(*max)}

	switch arg {
	case "histogram":
		graph := graphblast.NewHistogram(*bucket, *label, *wide)
		graph.Width = *width
		graph.Height = *height
		graph.Colors = *colors
		graph.FontSize = *fontSize
		graph.Allowed = allowed
		return graph
	case "timeseries":
		graph := graphblast.NewTimeSeries(*window, *label)
		graph.Width = *width
		graph.Height = *height
		graph.Colors = *colors
		graph.FontSize = *fontSize
		graph.Allowed = allowed
		return graph
	case "scatterplot":
		graph := graphblast.NewScatterPlot(*label)
		graph.Width = *width
		graph.Height = *height
		graph.Colors = *colors
		graph.FontSize = *fontSize
		graph.Allowed = allowed
		return graph
	case "logfile":
		graph := graphblast.NewLogFile(*window, *label)
		graph.Colors = *colors
		graph.FontSize = *fontSize
		return graph
	}
	panic("no graph for type")
}

func main() {
	flag.Parse()
	graphblast.SetVerboseLogging(*verbose)

	readerrors := make(chan error)
	watchers := make(graphblast.ErrorWatchers)
	go watchers.Broadcast(readerrors)

	// TODO Make graph-specific flags part of a subcommand/FlagSet
	graph := buildGraph(flag.Arg(0))
	go graph.Read(os.Stdin, readerrors)

	updateFreq := time.Duration(*delay) * time.Second

	http.HandleFunc("/", graphblast.Index())
	http.HandleFunc("/script.js", graphblast.Script())
	http.HandleFunc("/data", graphblast.Events(updateFreq, watchers, graph))

	graphblast.Log("listening on %v", *listen)
	http.ListenAndServe(*listen, nil)
}