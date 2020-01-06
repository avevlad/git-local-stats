package gdotfiles

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"

	"github.com/avevlad/gdotfiles/internal/build"
	"github.com/avevlad/gdotfiles/internal/config"
	"github.com/avevlad/gdotfiles/internal/logger"

	"github.com/avevlad/gdotfiles/internal/utils"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var errNoFilesFound = errors.New("No files found, try simplifying the arguments")

type App struct {
	Flags *AppFlags
}

type AppOption func(*App)

func NewApp(opts ...AppOption) *App {
	var appFlags AppFlags
	appFlags.RegisterFlags(flag.CommandLine)

	app := &App{}
	app.Flags = &appFlags

	for _, opt := range opts {
		opt(app)
	}

	return app
}

//func WithVerbose(verbose bool) AppOption {
//	return func(app *App) {
//		app.Verbose = verbose
//	}
//}

type AppFlags struct {
	Name    string
	Type    string
	From    string
	Verbose bool
	Yes     bool
}

func (af *AppFlags) RegisterFlags(fs *flag.FlagSet) {
	fs.StringVar(&af.Name, "name", "", "")
	fs.StringVar(&af.Type, "type", "", "")
	fs.StringVar(&af.From, "from", "", "")
	fs.BoolVar(&af.Verbose, "verbose", false, "")
	fs.BoolVar(&af.Yes, "yes", false, "")

	fs.Usage = func() {
		// print(HelpText())
	}
}

func (app *App) Run() error {
	//func Run() error {
	var (
		cfg   = config.NewConfig()
		files Files
	)

	setupDataDirs()
	cfg.Sync()

	//verbose := app.Verbose
	fmt.Println("appFlags 2", app.Flags)

	flag.Parse()
	fmt.Println("appFlags 3", app.Flags)
	var logLevel = zerolog.FatalLevel

	if app.Flags.Verbose {
		logLevel = zerolog.DebugLevel
	}
	logger.InitLogger(&logger.ConsoleLoggerOpts{Level: logLevel})

	log.Debug().Msg("some msg")
	log.Info().Strs("version", []string{build.Version, build.Revision}).Send()

	// println(build.Revision)
	// println(build.Version)

	// fmt.Println("CheckFzfExist", utils.CheckFzfExist())
	// fmt.Println("CheckGitExist", utils.CheckGitExist())

	downloadRepos(*cfg)
	files.Read(*cfg)

	if app.Flags.Name != "" {
		r := files.FilterByFlags(app.Flags)
		if r == (File{}) {
			return errNoFilesFound
		}
		offerFoundFile(r, app.Flags)
		return nil
	}

	var interactiveInput []string

	for _, v := range files.List {
		l := len(v.Name)
		left := v.Name + files.NameMaxTpl[l-1:]

		interactiveInput = append(interactiveInput, left+" ["+v.Folder+"]")
	}

	//runFZF(interactiveInput)

	fmt.Println("FINISH")
	return nil
}

func runFZF(input []string) string {
	var (
		bufOut = new(bytes.Buffer)
		cmd    = exec.Command("sh", "-c", "fzf")
	)

	cmd.Stdin = strings.NewReader(strings.Join(input, "\n"))
	cmd.Stdout = bufOut
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	utils.MustCheckWithLog(err, "fzf error")

	fmt.Println(strings.TrimSpace(bufOut.String()) == "bar")

	return bufOut.String()
}

func setupDataDirs() {
	appDir := utils.UserConfigDir()
	err := utils.MakeDirIfNotExists(appDir)
	utils.MustCheckWithLog(err, "setupDataDirs")

	err = utils.MakeDirIfNotExists(utils.GetCustomGitFilesFolderPath())
	utils.MustCheckWithLog(err, "setupDataDirs custom folder")
}

func downloadRepos(cfg config.Config) {
	if _, err := os.Stat(path.Join(utils.UserConfigDir(), "github_gitignore")); !os.IsNotExist(err) {
		// fmt.Println("not the first run")
		return
	}

	errChan := make(chan error)
	wg := sync.WaitGroup{}
	reposList := cfg.GetReposUrls()
	reposFolders := cfg.GetReposFolders()
	fmt.Println("This is the first run, we need some time to clone and cache gitignore and gitattribute files")

	for i, v := range reposList {
		wg.Add(1)
		go func(index int, url string) {
			defer wg.Done()

			folder := reposFolders[index]
			fmt.Println("Start cloning", url, "in", folder)

			log.Debug().Str("folder", folder).Msg("download start")
			cmd := exec.Command(`git`, `clone`, url, folder)
			cmd.Dir = utils.UserConfigDir()
			resp, err := cmd.CombinedOutput()
			if err != nil {
				if len(resp) > 0 {
					fmt.Println("err resp:", string(resp))
				}
				errChan <- err
			}
			log.Debug().Str("folder", folder).Str("resp", string(resp)).Msg("download finish")
		}(i, v)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	err := <-errChan
	utils.MustCheckWithLog(err, "git clone fatal err")
}
