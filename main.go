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
	"sync"
	"time"
)

// with sync for resource lock
type scratch struct {
	sync.RWMutex
	s *hyperscan.Scratch
}

var (
	Version  string
	Debug    bool
	Port     int
	FilePath string
	Flag     string
	Scratch  scratch
	Db       hyperscan.BlockDatabase
	Uptime   time.Time
	RegexMap map[int]RegexLine
)

type Response struct {
	Errno int         `json:errno`
	Msg   string      `json:msg`
	Data  interface{} `json:data`
}

type MatchResp struct {
	Id         int       `json:id`
	From       int       `json:from`
	To         int       `json:to`
	Flags      int       `json:flags`
	Context    string    `json:context`
	RegexLinev RegexLine `json:regexline`
}

type RegexLine struct {
	Expr string
	Data string
}

func main() {
	Version = "0.0.1"
	viper.AutomaticEnv()
	var rootCmd = &cobra.Command{
		Use:     "gohs-ladon",
		Short:   fmt.Sprintf("Gohs-ladon Service %s", Version),
		Run:     run,
		PreRunE: preRunE,
	}
	rootCmd.Flags().Bool("debug", false, "Enable debug mode")
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

	fmt.Printf("[%s] gohs-ladon %s Running on %s\n", Uptime.Format(time.RFC3339), Version, addr)
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
		log.SetLevel(log.InfoLevel)
	}
	log.Debug("Prerun", args)
	RegexMap = make(map[int]RegexLine)
	err := buildScratch(FilePath)
	return err
}

// build scratch for regex file.
func buildScratch(filepath string) (err error) {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	patterns := []*hyperscan.Pattern{}
	var expr hyperscan.Expression
	var id int
	//flags := Flag
	//flags := hyperscan.Caseless | hyperscan.Utf8Mode
	flags, err := hyperscan.ParseCompileFlag(Flag)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		log.Debug(scanner.Text())
		line := scanner.Text()
		// line start with #, skip
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			log.Info(fmt.Sprintf("line start with #, skip line: %s", line))
			continue
		}
		s := strings.Split(line, "\t")
		// length less than 3, skip
		if len(s) < 3 {
			log.Info(fmt.Sprintf("line length less than 3, skip line: %s", line))
			continue
		}
		id, err = strconv.Atoi(s[0])
		if err != nil {
			return fmt.Errorf("Atoi error.")
		}
		expr = hyperscan.Expression(s[1])
		data := s[2]
		pattern := &hyperscan.Pattern{Expression: expr, Flags: flags, Id: id}
		patterns = append(patterns, pattern)
		RegexMap[id] = RegexLine{string(expr), data}
	}
	if len(patterns) <= 0 {
		return fmt.Errorf("Empty regex")
	}
	log.Info(fmt.Sprintf("regex file line number: %d", len(patterns)))
	log.Info("Start Building, please wait...")
	db, err := hyperscan.NewBlockDatabase(patterns...)
	Db = db

	if err != nil {
		return err
	}
	scratch, err := hyperscan.NewScratch(Db)
	if err != nil {
		return err
	}
	Scratch.s = scratch

	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
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
			regexLine, ok := RegexMap[int(id)]
			if !ok {
				regexLine = RegexLine{}
			}
			matchResp := MatchResp{Id: int(id), From: int(from), To: int(to), Flags: int(flags), Context: fmt.Sprintf("%s", context), RegexLinev: regexLine}
			matchResps = append(matchResps, matchResp)
			return nil
		}

		// lock scratch
		Scratch.Lock()
		if err := Db.Scan(inputData, Scratch.s, eventHandler, inputData); err != nil {
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
		// unlock scratch
		Scratch.Unlock()
	}
	json.NewEncoder(w).Encode(resp)
	w.WriteHeader(http.StatusOK)
}

func statsHandle(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, fmt.Sprintf("gohs-ladon %v, Uptime %v",
		Version, Uptime.Format(time.RFC3339)))
}
