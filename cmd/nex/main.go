package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"nex-lang/pkg/client"
	"nex-lang/pkg/compiler"
	"nex-lang/pkg/config"
	"nex-lang/pkg/database"
	"nex-lang/pkg/evaluator"
	"nex-lang/pkg/host"
	"nex-lang/pkg/lexer"
	"nex-lang/pkg/manifest"
	"nex-lang/pkg/nextest"
	"nex-lang/pkg/parser"
	"nex-lang/pkg/repl"
	"nex-lang/pkg/runtime"
	"nex-lang/pkg/vm"
)

const modulesDir = ".modules"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "init":
		err = cmdInit(args)
	case "run", "serve":
		err = cmdRun(args)
	case "repl":
		err = cmdRepl(args)
	case "test":
		err = cmdTest(args)
	case "login":
		err = cmdLogin(args)
	case "logout":
		err = cmdLogout(args)
	case "publish":
		err = cmdPublish(args)
	case "install":
		err = cmdInstall(args)
	case "yank":
		err = cmdYank(args)
	case "help", "-h", "--help":
		printUsage()
		return
	case "version", "-v", "--version":
		fmt.Println("nex 0.3.0")
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `nex — Nexus language runtime and package manager

Usage:
  nex <command> [arguments]

Commands:
  init                      Create a default nexus.toml in the current directory
  run <file.nex>            Execute a Nexus program (starts HTTP server if routes registered)
  serve <file.nex>          Alias for run (HTTP apps)
  repl                      Interactive read-eval-print loop
  test [paths...]           Run *_test.nex / tests/**/*.nex files
  login                     Authenticate with the registry (API token or password)
  logout                    Clear stored registry credentials
  publish                   Bundle .nex + nexus.toml and publish to the registry
  install <pkg>[@<ver>]     Download and install a package into .modules/
  yank <pkg>@<ver>          Yank a published version (requires --reason)
  help                      Show this help message
  version                   Show version information

Run options:
  --vm                      Execute with the bytecode VM (core language; no import/host)

Registry config:
  File                      $CONFIG/nex/config.toml (registry_url, token)
  NEX_REGISTRY_URL          Registry base URL (default http://localhost:8080)
  NEX_TOKEN / NEX_API_KEY   Bearer token (nex_… API key or nxs_… session)

Examples:
  nex login --token nex_…
  nex login --browser
  nex publish
  nex install httpkit@1.2.0
  nex yank httpkit@1.2.0 --reason "critical bug"

Runtime environment (nex run / serve):
  DATABASE_URL, DATABASE_URL_READ, LISTEN_ADDR, STORAGE_DIR, BASE_URL,
  CDN_BASE_URL, MIGRATIONS_DIR, NEX_WEB_DIR, NEX_APP_DIR, NEX_STDLIB_DIR
`)
}

func cmdInit(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	path := filepath.Join(cwd, manifest.FileName)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", manifest.FileName)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	name := filepath.Base(cwd)
	author := os.Getenv("USER")
	if author == "" {
		author = os.Getenv("USERNAME")
	}
	if len(args) > 0 && args[0] != "" {
		name = args[0]
	}

	m := manifest.Default(name, author)
	if err := m.Write(path); err != nil {
		return err
	}

	fmt.Printf("Created %s for package %s@%s\n", manifest.FileName, m.Name, m.Version)
	return nil
}

func cmdRepl(args []string) error {
	engine := "tree"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--vm":
			engine = "vm"
		case "--engine":
			if i+1 >= len(args) {
				return fmt.Errorf("usage: nex repl [--vm|--engine tree|vm]")
			}
			i++
			engine = args[i]
			if engine != "tree" && engine != "vm" {
				return fmt.Errorf("unknown engine %q (want tree or vm)", engine)
			}
		default:
			return fmt.Errorf("usage: nex repl [--vm|--engine tree|vm]")
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	return repl.Run(os.Stdin, os.Stdout, repl.Config{
		Engine:    engine,
		RootDir:   cwd,
		StdlibDir: envOr("NEX_STDLIB_DIR", ""),
	})
}

func cmdTest(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	sum, err := nextest.Run(nextest.Options{
		RootDir:   cwd,
		StdlibDir: envOr("NEX_STDLIB_DIR", ""),
		Paths:     args,
		Out:       os.Stdout,
	})
	if err != nil {
		return err
	}
	if sum.Failed > 0 {
		return fmt.Errorf("%d test(s) failed", sum.Failed)
	}
	return nil
}

func cmdRun(args []string) error {
	useVM := false
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--vm":
			useVM = true
		case "--engine":
			if i+1 >= len(args) {
				return fmt.Errorf("usage: nex run [--vm] <file.nex>")
			}
			i++
			switch args[i] {
			case "vm":
				useVM = true
			case "tree":
				useVM = false
			default:
				return fmt.Errorf("unknown engine %q (want tree or vm)", args[i])
			}
		default:
			positional = append(positional, args[i])
		}
	}
	if len(positional) < 1 {
		return fmt.Errorf("usage: nex run [--vm] <file.nex>")
	}

	filename := positional[0]
	if useVM {
		return cmdRunVM(filename)
	}

	absFile, err := filepath.Abs(filename)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", filename, err)
	}
	if _, err := os.Stat(absFile); err != nil {
		return fmt.Errorf("read %s: %w", filename, err)
	}

	appDir := envOr("NEX_APP_DIR", filepath.Dir(absFile))
	if abs, err := filepath.Abs(appDir); err == nil {
		appDir = abs
	}

	webDir := envOr("NEX_WEB_DIR", "")
	if webDir == "" {
		// Prefer ./web from cwd, then sibling of app dir.
		candidates := []string{
			filepath.Join(".", "web"),
			filepath.Join(filepath.Dir(appDir), "web"),
			filepath.Join(appDir, "..", "web"),
		}
		for _, c := range candidates {
			if abs, err := filepath.Abs(c); err == nil {
				if st, err := os.Stat(filepath.Join(abs, "templates")); err == nil && st.IsDir() {
					webDir = abs
					break
				}
			}
		}
	} else if abs, err := filepath.Abs(webDir); err == nil {
		webDir = abs
	}

	storageDir := envOr("STORAGE_DIR", "./storage")
	storageDir = filepath.Clean(storageDir)
	if err := os.MkdirAll(storageDir, 0o750); err != nil {
		return fmt.Errorf("create storage directory: %w", err)
	}

	listenAddr := envOr("LISTEN_ADDR", ":8080")
	baseURL := envOr("BASE_URL", "http://localhost:8080")
	cdnBaseURL := envOr("CDN_BASE_URL", baseURL)
	maxUpload := int64(64 << 20)
	if raw := os.Getenv("MAX_UPLOAD_BYTES"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n <= 0 {
			return fmt.Errorf("MAX_UPLOAD_BYTES must be a positive integer")
		}
		maxUpload = n
	}
	publishCooldownMin := 30
	if raw := strings.TrimSpace(os.Getenv("PUBLISH_RATE_LIMIT_MINUTES")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return fmt.Errorf("PUBLISH_RATE_LIMIT_MINUTES must be an integer between 1 and 60")
		}
		if n > 60 {
			n = 60
		}
		publishCooldownMin = n
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	var db *database.DB
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		db, err = database.Connect(ctx, databaseURL)
		if err != nil {
			return fmt.Errorf("connect database: %w", err)
		}
		defer db.Close()

		if readURL := strings.TrimSpace(os.Getenv("DATABASE_URL_READ")); readURL != "" {
			if err := db.AttachReadReplica(ctx, readURL); err != nil {
				return fmt.Errorf("attach DATABASE_URL_READ: %w", err)
			}
			logger.Info("read replica attached", "database_url_read", "set")
		}

		migDir := database.ResolveMigrationsDir(
			filepath.Join(".", "migrations"),
			filepath.Join(filepath.Dir(appDir), "migrations"),
			filepath.Join(appDir, "..", "migrations"),
		)
		if migDir != "" {
			migCtx, migCancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer migCancel()
			if err := db.ApplyVersionedMigrations(migCtx, migDir); err != nil {
				return fmt.Errorf("apply versioned migrations from %s: %w", migDir, err)
			}
			logger.Info("versioned migrations applied", "dir", migDir)
		}
	}

	// Reset host-injected builtins between invocations in the same process (tests).
	evaluator.ExtraBuiltins = map[string]*evaluator.Builtin{}

	h := host.New(db, host.Config{
		StorageDir:              storageDir,
		BaseURL:                 baseURL,
		CDNBaseURL:              cdnBaseURL,
		MaxUploadBytes:          maxUpload,
		ListenAddr:              listenAddr,
		AppDir:                  appDir,
		WebDir:                  webDir,
		PublishRateLimitMinutes: publishCooldownMin,
	}, logger)

	stdlibDir := envOr("NEX_STDLIB_DIR", "")
	rt := runtime.NewWithOptions(appDir, h.Env, runtime.Options{StdlibDir: stdlibDir})
	entryName := filepath.Base(absFile)
	// Prefer loading by path relative to app dir when possible.
	loadPath := absFile
	if rel, err := filepath.Rel(appDir, absFile); err == nil && !strings.HasPrefix(rel, "..") {
		loadPath = rel
	} else {
		loadPath = entryName
		// When entry is outside appDir, set root to file dir.
		rt = runtime.NewWithOptions(filepath.Dir(absFile), h.Env, runtime.Options{StdlibDir: stdlibDir})
		loadPath = filepath.Base(absFile)
	}

	result := rt.LoadFile(loadPath)
	if errObj, ok := result.(*evaluator.Error); ok {
		return fmt.Errorf("load Nexus app: %s", errObj.Message)
	}

	forceServe := os.Args[1] == "serve" || envOr("NEX_FORCE_SERVE", "") == "1"
	if h.RouteCount == 0 && !forceServe {
		if result != nil && result != evaluator.NULL && result.Type() != evaluator.ErrorObj {
			fmt.Println(result.Inspect())
		}
		return nil
	}

	if db == nil {
		return fmt.Errorf("DATABASE_URL is required to serve HTTP apps that use the database (set e.g. postgres://postgres:postgres@localhost:5432/nex_registry?sslmode=disable)")
	}

	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           h.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("nex listening",
			"addr", listenAddr,
			"storage", storageDir,
			"base_url", baseURL,
			"cdn_base_url", cdnBaseURL,
			"app", appDir,
			"web", webDir,
			"routes", h.RouteCount,
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-signalCtx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("listen and serve: %w", err)
		}
		return nil
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	if err := <-errCh; err != nil {
		return err
	}
	logger.Info("server stopped cleanly")
	return nil
}

func cmdRunVM(filename string) error {
	absFile, err := filepath.Abs(filename)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", filename, err)
	}
	data, err := os.ReadFile(absFile)
	if err != nil {
		return fmt.Errorf("read %s: %w", filename, err)
	}
	l := lexer.New(string(data))
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return fmt.Errorf("parse error: %s", strings.Join(p.Errors(), "; "))
	}
	comp := compiler.New()
	if err := comp.Compile(program); err != nil {
		return fmt.Errorf("compile error: %w", err)
	}
	machine := vm.New(comp.Bytecode())
	if err := machine.Run(); err != nil {
		return fmt.Errorf("vm error: %w", err)
	}
	result := machine.LastPoppedStackElem()
	if result != nil && result != evaluator.NULL {
		fmt.Println(result.Inspect())
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func cmdLogin(args []string) error {
	var (
		tokenFlag   string
		userFlag    string
		browser     bool
		mintAPIKey  bool
		registryURL string
	)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--token" || arg == "-t":
			if i+1 >= len(args) {
				return fmt.Errorf("usage: nex login --token <nex_…|nxs_…>")
			}
			i++
			tokenFlag = args[i]
		case strings.HasPrefix(arg, "--token="):
			tokenFlag = strings.TrimPrefix(arg, "--token=")
		case arg == "--username" || arg == "-u":
			if i+1 >= len(args) {
				return fmt.Errorf("usage: nex login --username <name>")
			}
			i++
			userFlag = args[i]
		case strings.HasPrefix(arg, "--username="):
			userFlag = strings.TrimPrefix(arg, "--username=")
		case arg == "--browser" || arg == "-b":
			browser = true
		case arg == "--api-key":
			mintAPIKey = true
		case arg == "--registry":
			if i+1 >= len(args) {
				return fmt.Errorf("usage: nex login --registry <url>")
			}
			i++
			registryURL = args[i]
		case strings.HasPrefix(arg, "--registry="):
			registryURL = strings.TrimPrefix(arg, "--registry=")
		case arg == "-h" || arg == "--help":
			fmt.Print(`Usage:
  nex login [--token <token>] [--username <user>] [--browser] [--api-key] [--registry <url>]

Modes:
  --token      Store an API key (nex_…) or session token (nxs_…) directly
  --browser    Open the registry settings page and paste an API key
  (default)    Interactive username/password login via POST /api/auth/login

  --api-key    After password login, mint and store a long-lived API key instead of the session
`)
			return nil
		default:
			return fmt.Errorf("unknown login flag %q (try --help)", arg)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if registryURL != "" {
		cfg.RegistryURL = strings.TrimRight(registryURL, "/")
	}

	c := client.New(client.Options{BaseURL: cfg.RegistryBase(), Token: cfg.AuthToken()})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	switch {
	case tokenFlag != "":
		c.SetToken(tokenFlag)
		u, err := c.Profile(ctx)
		if err != nil {
			return fmt.Errorf("validate token: %w", err)
		}
		cfg.Token = tokenFlag
		cfg.Username = u.Username
		if err := cfg.Save(); err != nil {
			return err
		}
		path, _ := config.Path()
		fmt.Printf("Logged in as %s (token stored in %s)\n", u.Username, path)
		fmt.Printf("Registry: %s\n", cfg.RegistryBase())
		return nil

	case browser:
		settingsURL := cfg.RegistryBase() + "/settings"
		fmt.Printf("Open %s, generate an API key, then paste it below.\n", settingsURL)
		_ = openBrowser(settingsURL)
		token, err := promptLine("API key (nex_…): ")
		if err != nil {
			return err
		}
		if token == "" {
			return fmt.Errorf("empty token")
		}
		c.SetToken(token)
		u, err := c.Profile(ctx)
		if err != nil {
			return fmt.Errorf("validate token: %w", err)
		}
		cfg.Token = token
		cfg.Username = u.Username
		if err := cfg.Save(); err != nil {
			return err
		}
		path, _ := config.Path()
		fmt.Printf("Logged in as %s (token stored in %s)\n", u.Username, path)
		return nil

	default:
		login := userFlag
		if login == "" {
			login, err = promptLine("Username or email: ")
			if err != nil {
				return err
			}
		}
		password, err := promptPassword("Password: ")
		if err != nil {
			return err
		}
		if login == "" || password == "" {
			return fmt.Errorf("login and password are required")
		}
		result, err := c.Login(ctx, login, password)
		if err != nil {
			return err
		}
		token := result.Token
		username := result.User.Username
		if mintAPIKey {
			c.SetToken(token)
			key, err := c.CreateAPIKey(ctx, "cli")
			if err != nil {
				return fmt.Errorf("mint api key: %w", err)
			}
			token = key
			fmt.Println("Created API key labeled \"cli\" (session not stored).")
		}
		cfg.Token = token
		cfg.Username = username
		if err := cfg.Save(); err != nil {
			return err
		}
		path, _ := config.Path()
		fmt.Printf("Logged in as %s (credentials stored in %s)\n", username, path)
		fmt.Printf("Registry: %s\n", cfg.RegistryBase())
		return nil
	}
}

func cmdLogout(args []string) error {
	_ = args
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.ClearCredentials()
	if err := cfg.Save(); err != nil {
		return err
	}
	path, _ := config.Path()
	fmt.Printf("Cleared credentials in %s\n", path)
	return nil
}

func cmdPublish(args []string) error {
	_ = args
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	m, err := manifest.LoadFromDir(cwd)
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(cwd, manifest.FileName)

	tmpDir, err := os.MkdirTemp("", "nex-publish-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archiveName := fmt.Sprintf("%s-%s.nex", m.Name, m.Version)
	archivePath := filepath.Join(tmpDir, archiveName)

	fmt.Printf("Bundling package %s@%s...\n", m.Name, m.Version)
	if err := createNexArchive(cwd, archivePath); err != nil {
		return fmt.Errorf("bundle package: %w", err)
	}

	checksum, err := sha256File(archivePath)
	if err != nil {
		return err
	}
	fmt.Printf("SHA-256: sha256:%s\n", checksum)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	c := client.New(client.Options{})
	fmt.Printf("Publishing to %s...\n", c.BaseURL())

	readmePath := ""
	for _, candidate := range []string{"README.md", "Readme.md", "readme.md"} {
		p := filepath.Join(cwd, candidate)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			readmePath = p
			break
		}
	}

	result, err := c.Publish(ctx, client.PublishOptions{
		ManifestPath: manifestPath,
		PackagePath:  archivePath,
		ReadmePath:   readmePath,
	})
	if err != nil {
		return err
	}

	name := result.PackageName
	if name == "" {
		name = m.Name
	}
	ver := result.Version
	if ver == "" {
		ver = m.Version
	}
	if result.Message != "" {
		fmt.Println(result.Message)
	} else {
		fmt.Printf("Published %s@%s successfully\n", name, ver)
	}
	if result.DownloadURL != "" {
		fmt.Printf("Download: %s\n", result.DownloadURL)
	}
	return nil
}

func cmdInstall(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: nex install <package>[@version]")
	}

	name, version := parsePackageSpec(args[0])
	if name == "" {
		return fmt.Errorf("invalid package name")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	c := client.New(client.Options{})
	fmt.Printf("Querying %s for %s...\n", c.BaseURL(), args[0])
	info, err := c.ResolvePackage(ctx, name, version)
	if err != nil {
		return err
	}
	version = info.Version
	if info.Yanked {
		fmt.Fprintf(os.Stderr, "warning: %s@%s is yanked\n", name, version)
	}

	tmpDir, err := os.MkdirTemp("", "nex-install-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archiveName := info.Filename
	if archiveName == "" {
		archiveName = fmt.Sprintf("%s-%s.nex", name, version)
	}
	archivePath := filepath.Join(tmpDir, filepath.Base(archiveName))

	fmt.Printf("Downloading %s@%s...\n", name, version)
	if err := c.DownloadPackage(ctx, name, version, archivePath); err != nil {
		return err
	}

	actual, err := sha256File(archivePath)
	if err != nil {
		return err
	}
	expected := client.NormalizeChecksum(info.Checksum)
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("checksum mismatch for %s@%s: expected %s, got %s", name, version, expected, actual)
	}
	fmt.Println("Checksum verified")

	dest := filepath.Join(cwd, modulesDir, name)
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("clean install directory: %w", err)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("create modules directory: %w", err)
	}

	fmt.Printf("Extracting into %s...\n", dest)
	if err := extractNexArchive(archivePath, dest); err != nil {
		return fmt.Errorf("extract package: %w", err)
	}

	manifestPath := filepath.Join(cwd, manifest.FileName)
	if _, err := os.Stat(manifestPath); err == nil {
		m, err := manifest.Load(manifestPath)
		if err != nil {
			return fmt.Errorf("update local manifest: %w", err)
		}
		if m.Dependencies == nil {
			m.Dependencies = map[string]string{}
		}
		m.Dependencies[name] = version
		if err := m.Write(manifestPath); err != nil {
			return fmt.Errorf("write local manifest: %w", err)
		}
	}

	fmt.Printf("Installed %s@%s into %s\n", name, version, filepath.Join(modulesDir, name))
	return nil
}

func cmdYank(args []string) error {
	var (
		spec   string
		reason string
	)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--reason" || arg == "-r":
			if i+1 >= len(args) {
				return fmt.Errorf("usage: nex yank <pkg>@<ver> --reason <text>")
			}
			i++
			reason = args[i]
		case strings.HasPrefix(arg, "--reason="):
			reason = strings.TrimPrefix(arg, "--reason=")
		case arg == "-h" || arg == "--help":
			fmt.Print(`Usage:
  nex yank <package>@<version> --reason <text>

Marks a published version as yanked. Requires registry authentication.
`)
			return nil
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown yank flag %q", arg)
		default:
			if spec != "" {
				return fmt.Errorf("usage: nex yank <pkg>@<ver> --reason <text>")
			}
			spec = arg
		}
	}
	if spec == "" {
		return fmt.Errorf("usage: nex yank <pkg>@<ver> --reason <text>")
	}
	name, version := parsePackageSpec(spec)
	if name == "" || version == "" {
		return fmt.Errorf("yank requires <package>@<version>")
	}
	if strings.TrimSpace(reason) == "" {
		var err error
		reason, err = promptLine("Reason: ")
		if err != nil {
			return err
		}
	}
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("yank reason is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	c := client.New(client.Options{})
	fmt.Printf("Yanking %s@%s on %s...\n", name, version, c.BaseURL())
	result, err := c.Yank(ctx, name, version, reason)
	if err != nil {
		return err
	}
	if result.Message != "" {
		fmt.Println(result.Message)
	} else {
		fmt.Printf("Yanked %s@%s\n", result.Name, result.Version)
	}
	if result.Reason != "" {
		fmt.Printf("Reason: %s\n", result.Reason)
	}
	return nil
}

func parsePackageSpec(spec string) (name, version string) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", ""
	}
	if i := strings.LastIndex(spec, "@"); i > 0 {
		return spec[:i], spec[i+1:]
	}
	return spec, ""
}
