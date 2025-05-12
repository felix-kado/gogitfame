//go:build !solution

package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"gogitfame/configs"
)

type Config struct {
	Repository    string
	Revision      string
	OrderBy       string
	UseCommitter  bool
	Format        string
	Extensions    []string
	Languages     []string
	Exclude       []string
	RestrictTo    []string
	ExtensionsMap map[string][]string
}

type ActorStats struct {
	Name       string `json:"name"`
	Lines      int    `json:"lines"`
	commitsSet map[string]struct{}
	Commits    int `json:"commits"`
	Files      int `json:"files"`
}

func main() {
	var config Config

	var rootCmd = &cobra.Command{
		Use:   "gitfare",
		Short: "Collects statistics from a git repository",
		Run: func(cmd *cobra.Command, args []string) {
			config.ExtensionsMap = configs.LoadExtensionsMap()

			files := getFiles(config)
			filteredFiles := parallelFilter(files, config)
			actorStats := aggregateStats(filteredFiles, config)
			outputResults(actorStats, config)
		},
	}

	rootCmd.Flags().StringVar(&config.Repository, "repository", ".", "Path to the git repository")
	rootCmd.Flags().StringVar(&config.Revision, "revision", "HEAD", "Commit reference")
	rootCmd.Flags().StringVar(&config.OrderBy, "order-by", "lines", "Order of results: lines, commits, files")
	rootCmd.Flags().BoolVar(&config.UseCommitter, "use-committer", false, "Use committer instead of author")
	rootCmd.Flags().StringVar(&config.Format, "format", "tabular", "Output format: tabular, csv, json, json-lines")
	rootCmd.Flags().StringSliceVar(&config.Extensions, "extensions", []string{}, "List of file extensions to include")
	rootCmd.Flags().StringSliceVar(&config.Languages, "languages", []string{}, "List of languages to include")
	rootCmd.Flags().StringSliceVar(&config.Exclude, "exclude", []string{}, "Glob patterns to exclude files")
	rootCmd.Flags().StringSliceVar(&config.RestrictTo, "restrict-to", []string{}, "Glob patterns to restrict files to")

	cobra.OnInitialize(func() {
		validateConfig(&config, rootCmd.Flags())
	})

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func validateConfig(config *Config, flags *pflag.FlagSet) {
	validFormats := map[string]bool{"tabular": true, "csv": true, "json": true, "json-lines": true}
	if _, ok := validFormats[config.Format]; !ok {
		fmt.Fprintf(os.Stderr, "Invalid format: %s\n", config.Format)
		os.Exit(2)
	}

	validOrders := map[string]bool{"lines": true, "commits": true, "files": true}
	if _, ok := validOrders[config.OrderBy]; !ok {
		fmt.Fprintf(os.Stderr, "Invalid order-by value: %s\n", config.OrderBy)
		os.Exit(2)
	}

	cmd := exec.Command("git", "-C", config.Repository, "cat-file", "-e", config.Revision)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid revision: %s\n", config.Revision)
		os.Exit(2)
	}
}

func getFiles(config Config) []string {
	cmd := exec.Command("git", "-C", config.Repository, "ls-tree", "-r", "--name-only", config.Revision)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка выполнения команды git ls-tree: %v\n", err)
		return nil
	}

	var files []string

	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		file := scanner.Text()
		files = append(files, file)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка чтения вывода команды git ls-tree: %v\n", err)
		return nil
	}

	return files
}

func matchesExtensions(file string, extensions []string) bool {
	if len(extensions) == 0 {
		return true
	}

	for _, ext := range extensions {
		if strings.HasSuffix(file, ext) {
			return true
		}
	}
	return false
}

func matchesExcludePatterns(file string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}

	for _, pattern := range patterns {
		matched, _ := filepath.Match(pattern, file)
		if matched {
			return false
		}
	}
	return true
}

func matchesRestrictToPatterns(file string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}

	for _, pattern := range patterns {
		matched, _ := filepath.Match(pattern, file)
		if matched {
			return true
		}
	}
	return false
}

func matchesLanguage(filePath string, config Config) bool {
	if len(config.Languages) == 0 {
		return true
	}
	fileExtension := filepath.Ext(filePath)

	for _, allowedLang := range config.Languages {
		allowedLang = strings.ToLower(allowedLang)
		if existedLang, ok := config.ExtensionsMap[allowedLang]; ok {
			for _, allowedExtension := range existedLang {

				if strings.HasSuffix(fileExtension, allowedExtension) {
					return true
				}
			}
		}
	}

	return false
}

func parallelFilter(files []string, config Config) chan string {
	var filterWg sync.WaitGroup
	filteredChan := make(chan string, len(files))

	for _, file := range files {
		filterWg.Add(1)
		go func(file string) {
			defer filterWg.Done()
			if matchesExtensions(file, config.Extensions) &&
				matchesExcludePatterns(file, config.Exclude) &&
				matchesRestrictToPatterns(file, config.RestrictTo) &&
				matchesLanguage(file, config) {
				filteredChan <- file
			}
		}(file)
	}
	go func() {
		filterWg.Wait()
		close(filteredChan)
	}()

	return filteredChan
}

func infoEmptyFile(file string, config Config) ActorStats {
	cmd := exec.Command("git", "-C", config.Repository, "log", "-n", "1", "--pretty=format:%H\n%an", config.Revision, "--", file)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка выполнения команды git log: %v\n", err)
		return ActorStats{}
	}

	line := strings.Split(out.String(), "\n")
	commitHash, actor := line[0], line[1]
	stats := ActorStats{
		Name:       actor,
		Files:      1,
		commitsSet: make(map[string]struct{}),
	}
	stats.commitsSet[commitHash] = struct{}{}

	return stats
}

func calculateStats(file string, config Config) map[string]ActorStats {
	cmd := exec.Command("git", "-C", config.Repository, "blame", "--line-porcelain", file, config.Revision)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil
	}

	actorStats := make(map[string]ActorStats)

	if out.Len() == 0 {
		stats := infoEmptyFile(file, config)
		actorStats[stats.Name] = stats
		return actorStats
	}

	lines := strings.Split(out.String(), "\n")
	commitLineRegexp := regexp.MustCompile(`^\^?[a-f0-9]{40} \d+ \d+ \d+`)

	for i := 0; i < len(lines); i++ {
		if commitLineRegexp.MatchString(lines[i]) {
			parts := strings.Split(lines[i], " ")
			commitHash := parts[0]
			nLines, err := strconv.Atoi(parts[3])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Ошибка преобразования: %v\n", err)
				i++
				continue
			}
			actor := strings.TrimPrefix(lines[i+1], "author ")
			if config.UseCommitter {
				actor = strings.TrimPrefix(lines[i+5], "committer ")
			}

			if _, ok := actorStats[actor]; !ok {
				actorStats[actor] = ActorStats{
					Files:      1,
					commitsSet: make(map[string]struct{}),
				}
			}
			stats := actorStats[actor]
			stats.Lines += nLines
			stats.Name = actor
			stats.commitsSet[commitHash] = struct{}{}
			actorStats[actor] = stats
		}
	}

	return actorStats
}

func aggregateStats(files chan string, config Config) map[string]ActorStats {
	var aggWg sync.WaitGroup
	resultsChan := make(chan map[string]ActorStats)

	finalStats := make(map[string]ActorStats)

	for file := range files {
		aggWg.Add(1)
		go func(file string) {
			defer aggWg.Done()

			fileStats := calculateStats(file, config)
			resultsChan <- fileStats
		}(file)
	}

	go func() {
		aggWg.Wait()
		close(resultsChan)
	}()

	for stats := range resultsChan {
		for actor, info := range stats {
			if existing, ok := finalStats[actor]; ok {
				existing.Lines += info.Lines
				existing.Files += info.Files
				for commit := range info.commitsSet {
					existing.commitsSet[commit] = struct{}{}
				}
				finalStats[actor] = existing
			} else {
				finalStats[actor] = info
			}
		}
	}

	for actor, stats := range finalStats {
		stats.Commits = len(stats.commitsSet)
		finalStats[actor] = stats
	}

	return finalStats
}

func sortByConfig(actors []ActorStats, orderBy string) {
	sort.Slice(actors, func(i, j int) bool {
		if actors[i].Commits == actors[j].Commits &&
			actors[i].Lines == actors[j].Lines &&
			actors[i].Files == actors[j].Files {
			return actors[i].Name < actors[j].Name
		}

		switch orderBy {
		case "commits":
			if actors[i].Commits != actors[j].Commits {
				return actors[i].Commits > actors[j].Commits
			}
			if actors[i].Lines != actors[j].Lines {
				return actors[i].Lines > actors[j].Lines
			}

			return actors[i].Files > actors[j].Files
		case "files":
			if actors[i].Files != actors[j].Files {
				return actors[i].Files > actors[j].Files
			}
			if actors[i].Lines != actors[j].Lines {
				return actors[i].Lines > actors[j].Lines
			}
			return actors[i].Commits > actors[j].Commits
		default:
			if actors[i].Lines != actors[j].Lines {
				return actors[i].Lines > actors[j].Lines
			}
			if actors[i].Commits != actors[j].Commits {
				return actors[i].Commits > actors[j].Commits
			}
			return actors[i].Files > actors[j].Files
		}
	})
}

func outputResults(stats map[string]ActorStats, config Config) {
	actors := make([]ActorStats, 0, len(stats))
	for _, stat := range stats {
		actors = append(actors, stat)
	}

	sortByConfig(actors, config.OrderBy)

	switch config.Format {
	case "tabular":
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
		fmt.Fprintln(w, "Name\tLines\tCommits\tFiles")
		for _, actor := range actors {
			fmt.Fprintf(w, "%s\t%d\t%d\t%d\n", actor.Name, actor.Lines, actor.Commits, actor.Files)
		}
		w.Flush()
	case "csv":
		w := csv.NewWriter(os.Stdout)
		err := w.Write([]string{"Name", "Lines", "Commits", "Files"})
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Writing error: %s", err)
			os.Exit(1)
		}
		for _, actor := range actors {
			err = w.Write([]string{actor.Name, fmt.Sprint(actor.Lines), fmt.Sprint(actor.Commits), fmt.Sprint(actor.Files)})
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Writing error: %s", err)
				os.Exit(1)
			}
		}
		w.Flush()
	case "json":
		err := json.NewEncoder(os.Stdout).Encode(actors)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Writing error: %s", err)
			os.Exit(1)
		}
	case "json-lines":
		for _, actor := range actors {
			err := json.NewEncoder(os.Stdout).Encode(actor)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Writing error: %s", err)
				os.Exit(1)
			}
		}
	}
}
