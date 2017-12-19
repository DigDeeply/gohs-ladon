package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/flier/gohs/hyperscan"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	Version  string
	Debug    bool
	Port     int
	FilePath string
	Flag     string
	Scratch  *hyperscan.Scratch
	Db       hyperscan.BlockDatabase
	Uptime   time.Time
)

type Response struct {
	Errno int         `json:errno`
	Msg   string      `json:msg`
	Data  interface{} `json:data`
}

type MatchResp struct {
	Id      int
	From    int
	To      int
	Flags   int
	Context string
}

func main() {
	Version = "0.0.1"
	viper.AutomaticEnv()
	var rootCmd = &cobra.Command{
		Use:     "gohs",
		Short:   fmt.Sprintf("Gohs Service %s", Version),
		Run:     run,
		PreRunE: preRunE,
	}
	rootCmd.Flags().Bool("debug", true, "Enable debug mode")
	rootCmd.Flags().Int("port", 8080, "Listen port")
	rootCmd.Flags().String("filepath", "", "Dict file path")
	rootCmd.Flags().String("flag", "iou", "Regex Flag")

	viper.BindPFlag("debug", rootCmd.Flags().Lookup("debug"))
	viper.BindPFlag("port", rootCmd.Flags().Lookup("port"))
	viper.BindPFlag("filepath", rootCmd.Flags().Lookup("filepath"))
	viper.BindPFlag("flag", rootCmd.Flags().Lookup("flag"))

	rootCmd.Execute()
}

func run(cmd *cobra.Command, args []string) {

	// Todo add a goroutine to check if pattern file changed, and reload file.

	// start web service
	http.Handle("/", middleware(http.HandlerFunc(matchHandle)))
	http.Handle("/_stats", middleware(http.HandlerFunc(statsHandle)))

	addr := fmt.Sprintf("0.0.0.0:%d", Port)
	s := &http.Server{
		Addr:         addr,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
	}
	Uptime = time.Now()

	fmt.Printf("[%s] Hs-service %s Running on %s\n", Uptime.Format(time.RFC3339), Version, addr)
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}

}

func preRunE(cmd *cobra.Command, args []string) error {
	Debug = viper.GetBool("debug")
	Port = viper.GetInt("port")
	FilePath = viper.GetString("filepath")
	Flag = viper.GetString("flag")

	if FilePath == "" {
		return fmt.Errorf("empty regex filepath")
	}
	if Debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.WarnLevel)
	}
	log.Info("Prerun", args)
	_, _ = buildScratch(FilePath)
	return nil
}

// build scratch for regex file.
func buildScratch(filepath string) (scratch *hyperscan.Scratch, err error) {
	file, err := os.Open(filepath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	patterns := []*hyperscan.Pattern{}
	var expr hyperscan.Expression
	var id int
	//flags := Flag
	//flags := hyperscan.Caseless | hyperscan.Utf8Mode
	flags, err := hyperscan.ParseCompileFlag(Flag)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		log.Info(scanner.Text())
		line := scanner.Text()
		s := strings.Split(line, "\t")
		//fmt.Println(s, len(s))
		if len(s) != 2 {
			continue
		}
		id, err = strconv.Atoi(s[0])
		if err != nil {
			return nil, fmt.Errorf("Atoi error.")
		}
		expr = hyperscan.Expression(s[1])
		pattern := &hyperscan.Pattern{Expression: expr, Flags: flags, Id: id}
		patterns = append(patterns, pattern)
	}
	if len(patterns) <= 0 {
		log.Error("empty regex")
		os.Exit(-1)
	}
	log.Info(fmt.Sprintf("regex file line number: %d", len(patterns)))
	db, err := hyperscan.NewBlockDatabase(patterns...)
	Db = db

	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	scratch, err = hyperscan.NewScratch(Db)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	Scratch = scratch

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	return
}

func middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		end := time.Now()
		latency := end.Sub(start)
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		log.WithFields(log.Fields{
			"remote_addr":    host,
			"latency":        latency,
			"content_length": r.ContentLength,
		}).Info(fmt.Sprintf("%s %s", r.Method, r.RequestURI))
	})
}

func matchHandle(w http.ResponseWriter, r *http.Request) {
	query := r.FormValue("q")
	var resp Response = Response{Errno: 0}
	w.Header().Set("Content-Type", "application/json")
	if query == "" {
		resp.Errno = -1
		resp.Msg = "empty param q"
	} else {
		inputData := []byte(query)
		// results
		var matchResps []MatchResp
		eventHandler := func(id uint, from, to uint64, flags uint, context interface{}) error {
			log.Info(fmt.Sprintf("id: %d, from: %d, to: %d, flags: %v, context: %s", id, from, to, flags, context))
			matchResp := MatchResp{Id: int(id), From: int(from), To: int(to), Flags: int(flags), Context: fmt.Sprintf("%s", context)}
			matchResps = append(matchResps, matchResp)
			return nil
		}

		if err := Db.Scan(inputData, Scratch, eventHandler, inputData); err != nil {
			logFields := log.Fields{"query": query}
			log.WithFields(logFields).Error(err)
			resp.Errno = -2
			resp.Msg = fmt.Sprintf("Db.Scan error: %s", err)
		} else {
			if len(matchResps) <= 0 {
				resp.Errno = 1
				resp.Msg = "no match"
			}
			resp.Data = matchResps
		}
	}
	json.NewEncoder(w).Encode(resp)
	w.WriteHeader(http.StatusOK)
}

func statsHandle(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, fmt.Sprintf("Hs-service %v, Uptime %v",
		Version, Uptime.Format(time.RFC3339)))
}
