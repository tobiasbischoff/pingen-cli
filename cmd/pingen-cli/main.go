package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"pingen-cli/internal/pingen"
)

const version = "0.1.0"

const defaultScope = "letter batch webhook organisation_read"

func main() {
	exitCode := run(os.Args[1:])
	os.Exit(exitCode)
}

func run(args []string) int {
	global, subcommand, subargs, ok := parseGlobal(args)
	if !ok {
		return 2
	}
	if global.showVersion {
		fmt.Printf("pingen-cli %s\n", version)
		return 0
	}
	if subcommand == "" {
		printUsage()
		return 2
	}

	if global.plain {
		global.jsonOutput = false
	}

	configPath, err := pingen.ConfigPath()
	if err != nil {
		printError("failed to resolve config path", 0, "")
		return 1
	}

	cfg, cfgExists, cfgErr := pingen.LoadConfig(configPath)
	if cfgErr != nil && !errors.Is(cfgErr, os.ErrNotExist) {
		printError("failed to load config", 0, "")
		return 1
	}

	envCfg := configFromEnv()
	cliCfg := configFromGlobal(global)
	settings := pingen.MergeConfig(cfg, envCfg)
	settings = pingen.MergeConfig(settings, cliCfg)

	if global.clientSecretFile != "" {
		secret, err := os.ReadFile(global.clientSecretFile)
		if err != nil {
			printError("failed to read client secret file", 0, "")
			return 1
		}
		settings.ClientSecret = strings.TrimSpace(string(secret))
	}

	if settings.Env == "" {
		settings.Env = "staging"
	}
	if settings.Env != "staging" && settings.Env != "production" {
		printError("invalid env (use staging or production)", 0, "")
		return 2
	}
	settings = applyDefaultBases(settings)

	ctx := appContext{
		global:       global,
		configPath:   configPath,
		configLoaded: cfgExists,
		settings:     settings,
	}

	switch subcommand {
	case "auth":
		return handleAuth(ctx, subargs)
	case "config":
		return handleConfig(ctx, subargs)
	case "org":
		return handleOrg(ctx, subargs)
	case "letters":
		return handleLetters(ctx, subargs)
	default:
		printUsage()
		return 2
	}
}

type globalOptions struct {
	showHelp         bool
	showVersion      bool
	env              string
	apiBase          string
	identityBase     string
	organisationID   string
	accessToken      string
	clientID         string
	clientSecret     string
	clientSecretFile string
	timeout          int
	jsonOutput       bool
	plain            bool
	quiet            bool
	verbose          bool
	dryRun           bool
}

type appContext struct {
	global       globalOptions
	configPath   string
	configLoaded bool
	settings     pingen.Config
}

func parseGlobal(args []string) (globalOptions, string, []string, bool) {
	global := globalOptions{}
	fs := flag.NewFlagSet("pingen-cli", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.BoolVar(&global.showHelp, "help", false, "show help")
	fs.BoolVar(&global.showHelp, "h", false, "show help")
	fs.BoolVar(&global.showVersion, "version", false, "show version")
	fs.StringVar(&global.env, "env", "", "API environment (default: staging)")
	fs.StringVar(&global.apiBase, "api-base", "", "Override API base URL")
	fs.StringVar(&global.identityBase, "identity-base", "", "Override identity base URL")
	fs.StringVar(&global.organisationID, "org", "", "Organisation UUID")
	fs.StringVar(&global.accessToken, "access-token", "", "Access token (prefer env PINGEN_ACCESS_TOKEN)")
	fs.StringVar(&global.clientID, "client-id", "", "OAuth client id (prefer env PINGEN_CLIENT_ID)")
	fs.StringVar(&global.clientSecret, "client-secret", "", "OAuth client secret (prefer env/file over flags)")
	fs.StringVar(&global.clientSecretFile, "client-secret-file", "", "Read client secret from file")
	fs.IntVar(&global.timeout, "timeout", 30, "HTTP timeout seconds (default: 30)")
	fs.BoolVar(&global.jsonOutput, "json", false, "Output JSON")
	fs.BoolVar(&global.plain, "plain", false, "Output plain text (default)")
	fs.BoolVar(&global.quiet, "quiet", false, "Suppress non-essential output")
	fs.BoolVar(&global.verbose, "verbose", false, "Verbose output")
	fs.BoolVar(&global.dryRun, "dry-run", false, "Preview actions without sending")

	if err := fs.Parse(args); err != nil {
		return global, "", nil, false
	}
	if global.showHelp {
		printUsage()
		return global, "", nil, false
	}
	remaining := fs.Args()
	if len(remaining) == 0 {
		return global, "", nil, true
	}
	return global, remaining[0], remaining[1:], true
}

func printUsage() {
	fmt.Println(`pingen-cli - Send letters through Pingen from the command line.

Usage:
  pingen-cli [global flags] <command> [args]

Commands:
  auth token         Fetch an access token
  config show        Show config
  config set         Set config value
  config unset       Unset config value
  org list           List organisations
  letters list       List letters
  letters get        Get a letter
  letters create     Create a letter
  letters send       Send a letter

Global flags:
  --env <production|staging>
  --api-base <url>
  --identity-base <url>
  --org <uuid>
  --access-token <token>
  --client-id <id>
  --client-secret <secret>
  --client-secret-file <path>
  --timeout <seconds>
  --json | --plain
  --quiet | --verbose
  --dry-run
  -h, --help
  --version

Use "pingen-cli <command> --help" for command-specific options.`)
}

func configFromEnv() pingen.Config {
	cfg := pingen.Config{}
	if value := os.Getenv("PINGEN_ENV"); value != "" {
		cfg.Env = value
	}
	if value := os.Getenv("PINGEN_API_BASE"); value != "" {
		cfg.APIBase = value
	}
	if value := os.Getenv("PINGEN_IDENTITY_BASE"); value != "" {
		cfg.IdentityBase = value
	}
	if value := os.Getenv("PINGEN_ORG_ID"); value != "" {
		cfg.OrganisationID = value
	}
	if value := os.Getenv("PINGEN_ACCESS_TOKEN"); value != "" {
		cfg.AccessToken = value
	}
	if value := os.Getenv("PINGEN_CLIENT_ID"); value != "" {
		cfg.ClientID = value
	}
	if value := os.Getenv("PINGEN_CLIENT_SECRET"); value != "" {
		cfg.ClientSecret = value
	}
	return cfg
}

func configFromGlobal(global globalOptions) pingen.Config {
	return pingen.Config{
		Env:            global.env,
		APIBase:        global.apiBase,
		IdentityBase:   global.identityBase,
		OrganisationID: global.organisationID,
		AccessToken:    global.accessToken,
		ClientID:       global.clientID,
		ClientSecret:   global.clientSecret,
	}
}

func applyDefaultBases(cfg pingen.Config) pingen.Config {
	if cfg.APIBase == "" {
		if cfg.Env == "production" {
			cfg.APIBase = "https://api.pingen.com"
		} else {
			cfg.APIBase = "https://api-staging.pingen.com"
		}
	}
	if cfg.IdentityBase == "" {
		if cfg.Env == "production" {
			cfg.IdentityBase = "https://identity.pingen.com"
		} else {
			cfg.IdentityBase = "https://identity-staging.pingen.com"
		}
	}
	return cfg
}

func handleConfig(ctx appContext, args []string) int {
	if len(args) == 0 {
		fmt.Println("config requires a subcommand (show/set/unset)")
		return 2
	}
	switch args[0] {
	case "show":
		cfg, _, err := pingen.LoadConfig(ctx.configPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			printError("failed to load config", 0, "")
			return 1
		}
		return emitJSON(cfg)
	case "set":
		if len(args) < 3 {
			fmt.Println("config set requires key and value")
			return 2
		}
		cfg, _, _ := pingen.LoadConfig(ctx.configPath)
		switch args[1] {
		case "env":
			cfg.Env = args[2]
		case "api_base":
			cfg.APIBase = args[2]
		case "identity_base":
			cfg.IdentityBase = args[2]
		case "organisation_id":
			cfg.OrganisationID = args[2]
		case "access_token":
			cfg.AccessToken = args[2]
		case "client_id":
			cfg.ClientID = args[2]
		case "client_secret":
			cfg.ClientSecret = args[2]
		default:
			fmt.Printf("unknown config key: %s\n", args[1])
			return 2
		}
		if err := pingen.SaveConfig(ctx.configPath, cfg); err != nil {
			printError("failed to save config", 0, "")
			return 1
		}
		if !ctx.global.quiet {
			fmt.Printf("set %s\n", args[1])
		}
		return 0
	case "unset":
		if len(args) < 2 {
			fmt.Println("config unset requires key")
			return 2
		}
		cfg, _, _ := pingen.LoadConfig(ctx.configPath)
		switch args[1] {
		case "env":
			cfg.Env = ""
		case "api_base":
			cfg.APIBase = ""
		case "identity_base":
			cfg.IdentityBase = ""
		case "organisation_id":
			cfg.OrganisationID = ""
		case "access_token":
			cfg.AccessToken = ""
		case "client_id":
			cfg.ClientID = ""
		case "client_secret":
			cfg.ClientSecret = ""
		default:
			fmt.Printf("unknown config key: %s\n", args[1])
			return 2
		}
		if err := pingen.SaveConfig(ctx.configPath, cfg); err != nil {
			printError("failed to save config", 0, "")
			return 1
		}
		if !ctx.global.quiet {
			fmt.Printf("unset %s\n", args[1])
		}
		return 0
	default:
		fmt.Println("unknown config subcommand")
		return 2
	}
}

func handleAuth(ctx appContext, args []string) int {
	if len(args) == 0 {
		fmt.Println("auth requires a subcommand")
		return 2
	}
	if args[0] != "token" {
		fmt.Println("unknown auth subcommand")
		return 2
	}
	fs := flag.NewFlagSet("auth token", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	scope := fs.String("scope", defaultScope, "OAuth scope")
	save := fs.Bool("save", false, "Save token in config")
	saveCreds := fs.Bool("save-credentials", false, "Save client id/secret in config")
	help := fs.Bool("help", false, "show help")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if *help {
		fmt.Println("Usage: pingen-cli auth token [--scope ...] [--save] [--save-credentials]")
		return 0
	}
	if ctx.settings.ClientID == "" || ctx.settings.ClientSecret == "" {
		printError("client id/secret required", 0, "")
		return 2
	}
	client := pingen.Client{
		APIBase:      ctx.settings.APIBase,
		IdentityBase: ctx.settings.IdentityBase,
		Timeout:      time.Duration(ctx.global.timeout) * time.Second,
	}
	payload, _, err := client.GetToken(ctx.settings.ClientID, ctx.settings.ClientSecret, *scope)
	if err != nil {
		printError(err.Error(), 0, "")
		return 1
	}
	if *save || *saveCreds {
		cfg, _, _ := pingen.LoadConfig(ctx.configPath)
		cfg.Env = ctx.settings.Env
		cfg.APIBase = ctx.settings.APIBase
		cfg.IdentityBase = ctx.settings.IdentityBase
		if *save {
			if token, ok := payload["access_token"].(string); ok {
				cfg.AccessToken = token
			}
			if expires, ok := payload["expires_in"].(float64); ok {
				cfg.AccessTokenExpiresAt = time.Now().Add(time.Duration(int64(expires)) * time.Second).Unix()
			}
		}
		if *saveCreds {
			cfg.ClientID = ctx.settings.ClientID
			cfg.ClientSecret = ctx.settings.ClientSecret
		}
		if err := pingen.SaveConfig(ctx.configPath, cfg); err != nil {
			printError("failed to save config", 0, "")
			return 1
		}
	}
	return emitJSON(payload)
}

func handleOrg(ctx appContext, args []string) int {
	if len(args) == 0 {
		fmt.Println("org requires a subcommand")
		return 2
	}
	if args[0] != "list" {
		fmt.Println("unknown org subcommand")
		return 2
	}
	fs := flag.NewFlagSet("org list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	page := fs.Int("page", 0, "Page number")
	limit := fs.Int("limit", 0, "Page size")
	sort := fs.String("sort", "", "Sort expression")
	filter := fs.String("filter", "", "Filter JSON string or @path")
	query := fs.String("q", "", "Full-text query")
	include := fs.String("include", "", "Include relationships")
	fields := fs.String("fields", "", "Sparse fieldset for primary type")
	help := fs.Bool("help", false, "show help")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if *help {
		fmt.Println("Usage: pingen-cli org list [--page N] [--limit N] [--sort expr] [--filter json] [--q query] [--include rel] [--fields list]")
		return 0
	}

	params := buildListParams(*page, *limit, *sort, *filter, *query, *include, *fields, "organisations")
	token, err := ensureAccessToken(&ctx)
	if err != nil {
		printError(err.Error(), 0, "")
		return 1
	}
	client := pingen.Client{
		APIBase:     ctx.settings.APIBase,
		AccessToken: token,
		Timeout:     time.Duration(ctx.global.timeout) * time.Second,
	}
	payload, _, err := client.ListOrganisations(params)
	if err != nil {
		printError(err.Error(), 0, "")
		return 1
	}
	if ctx.global.jsonOutput {
		return emitJSON(payload)
	}
	data, _ := payload["data"].([]any)
	for _, entry := range data {
		item, _ := entry.(map[string]any)
		attrs, _ := item["attributes"].(map[string]any)
		fmt.Printf("%s\t%s\t%s\n", stringValue(item["id"]), stringValue(attrs["name"]), stringValue(attrs["status"]))
	}
	return 0
}

func handleLetters(ctx appContext, args []string) int {
	if len(args) == 0 {
		fmt.Println("letters requires a subcommand")
		return 2
	}
	sub := args[0]
	switch sub {
	case "list":
		return handleLettersList(ctx, args[1:])
	case "get":
		return handleLettersGet(ctx, args[1:])
	case "create":
		return handleLettersCreate(ctx, args[1:])
	case "send":
		return handleLettersSend(ctx, args[1:])
	default:
		fmt.Println("unknown letters subcommand")
		return 2
	}
}

func handleLettersList(ctx appContext, args []string) int {
	if ctx.settings.OrganisationID == "" {
		printError("organisation id required", 0, "")
		return 2
	}
	fs := flag.NewFlagSet("letters list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	page := fs.Int("page", 0, "Page number")
	limit := fs.Int("limit", 0, "Page size")
	sort := fs.String("sort", "", "Sort expression")
	filter := fs.String("filter", "", "Filter JSON string or @path")
	query := fs.String("q", "", "Full-text query")
	include := fs.String("include", "", "Include relationships")
	fields := fs.String("fields", "", "Sparse fieldset for primary type")
	help := fs.Bool("help", false, "show help")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *help {
		fmt.Println("Usage: pingen-cli letters list [--page N] [--limit N] [--sort expr] [--filter json] [--q query] [--include rel] [--fields list]")
		return 0
	}

	params := buildListParams(*page, *limit, *sort, *filter, *query, *include, *fields, "letters")
	token, err := ensureAccessToken(&ctx)
	if err != nil {
		printError(err.Error(), 0, "")
		return 1
	}
	client := pingen.Client{
		APIBase:     ctx.settings.APIBase,
		AccessToken: token,
		Timeout:     time.Duration(ctx.global.timeout) * time.Second,
	}
	payload, _, err := client.ListLetters(ctx.settings.OrganisationID, params)
	if err != nil {
		printError(err.Error(), 0, "")
		return 1
	}
	if ctx.global.jsonOutput {
		return emitJSON(payload)
	}
	data, _ := payload["data"].([]any)
	for _, entry := range data {
		item, _ := entry.(map[string]any)
		attrs, _ := item["attributes"].(map[string]any)
		fmt.Printf("%s\t%s\t%s\n", stringValue(item["id"]), stringValue(attrs["status"]), stringValue(attrs["file_original_name"]))
	}
	return 0
}

func handleLettersGet(ctx appContext, args []string) int {
	if ctx.settings.OrganisationID == "" {
		printError("organisation id required", 0, "")
		return 2
	}
	if len(args) == 0 {
		fmt.Println("letters get requires a letter id")
		return 2
	}
	letterID := args[0]
	token, err := ensureAccessToken(&ctx)
	if err != nil {
		printError(err.Error(), 0, "")
		return 1
	}
	client := pingen.Client{
		APIBase:     ctx.settings.APIBase,
		AccessToken: token,
		Timeout:     time.Duration(ctx.global.timeout) * time.Second,
	}
	payload, _, err := client.GetLetter(ctx.settings.OrganisationID, letterID)
	if err != nil {
		printError(err.Error(), 0, "")
		return 1
	}
	if ctx.global.jsonOutput {
		return emitJSON(payload)
	}
	item, _ := payload["data"].(map[string]any)
	attrs, _ := item["attributes"].(map[string]any)
	fmt.Println(stringValue(item["id"]))
	fmt.Printf("status: %s\n", stringValue(attrs["status"]))
	fmt.Printf("file: %s\n", stringValue(attrs["file_original_name"]))
	return 0
}

func handleLettersCreate(ctx appContext, args []string) int {
	if ctx.settings.OrganisationID == "" {
		printError("organisation id required", 0, "")
		return 2
	}
	fs := flag.NewFlagSet("letters create", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	filePath := fs.String("file", "", "PDF file to upload")
	fileName := fs.String("file-name", "", "Original file name shown in Pingen")
	addressPos := fs.String("address-position", "left", "Address position (left/right)")
	autoSend := fs.Bool("auto-send", false, "Automatically send when processed")
	deliveryProduct := fs.String("delivery-product", "", "Delivery product")
	printMode := fs.String("print-mode", "", "Print mode")
	printSpectrum := fs.String("print-spectrum", "", "Print spectrum")
	metaJSON := fs.String("meta-json", "", "Meta data JSON string or @path")
	metaFile := fs.String("meta-file", "", "Meta data JSON file path")
	idempotencyKey := fs.String("idempotency-key", "", "Idempotency key for create request")
	help := fs.Bool("help", false, "show help")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *help {
		fmt.Println("Usage: pingen-cli letters create --file <path> [--file-name name] [--address-position left|right] [--auto-send] [--delivery-product ...] [--print-mode ...] [--print-spectrum ...] [--meta-json ...|--meta-file ...] [--idempotency-key ...]")
		return 0
	}
	if *filePath == "" {
		printError("--file is required", 0, "")
		return 2
	}
	if *addressPos != "left" && *addressPos != "right" {
		printError("address-position must be left or right", 0, "")
		return 2
	}
	if _, err := os.Stat(*filePath); err != nil {
		printError("file not found", 0, "")
		return 2
	}
	originalName := *fileName
	if originalName == "" {
		originalName = pingen.DefaultFileName(*filePath)
	}
	metaData, err := loadJSONInput(*metaJSON, *metaFile)
	if err != nil {
		printError(err.Error(), 0, "")
		return 2
	}

	attributes := map[string]any{
		"file_original_name": originalName,
		"address_position":   *addressPos,
		"auto_send":          *autoSend,
	}
	if *deliveryProduct != "" {
		if !isAllowed(*deliveryProduct, []string{"fast", "cheap", "bulk", "premium", "registered"}) {
			printError("invalid delivery-product", 0, "")
			return 2
		}
		attributes["delivery_product"] = *deliveryProduct
	}
	if *printMode != "" {
		if !isAllowed(*printMode, []string{"simplex", "duplex"}) {
			printError("invalid print-mode", 0, "")
			return 2
		}
		attributes["print_mode"] = *printMode
	}
	if *printSpectrum != "" {
		if !isAllowed(*printSpectrum, []string{"color", "grayscale"}) {
			printError("invalid print-spectrum", 0, "")
			return 2
		}
		attributes["print_spectrum"] = *printSpectrum
	}
	if metaData != nil {
		attributes["meta_data"] = metaData
	}

	if ctx.global.dryRun {
		payload := map[string]any{
			"action":          "letters.create",
			"file":            *filePath,
			"organisation_id": ctx.settings.OrganisationID,
			"attributes":      attributes,
		}
		return emitJSON(payload)
	}

	token, err := ensureAccessToken(&ctx)
	if err != nil {
		printError(err.Error(), 0, "")
		return 1
	}
	client := pingen.Client{
		APIBase:     ctx.settings.APIBase,
		AccessToken: token,
		Timeout:     time.Duration(ctx.global.timeout) * time.Second,
	}
	if ctx.global.verbose && !ctx.global.quiet {
		fmt.Fprintln(os.Stderr, "requesting upload url...")
	}
	uploadURL, signature, _, err := client.GetFileUpload()
	if err != nil {
		printError(err.Error(), 0, "")
		return 1
	}
	if ctx.global.verbose && !ctx.global.quiet {
		fmt.Fprintln(os.Stderr, "uploading file...")
	}
	uploadTimeout := time.Duration(ctx.global.timeout) * time.Second
	if uploadTimeout < 60*time.Second {
		uploadTimeout = 60 * time.Second
	}
	if err := client.UploadFile(uploadURL, *filePath, uploadTimeout); err != nil {
		printError(err.Error(), 0, "")
		return 1
	}

	payload := map[string]any{
		"data": map[string]any{
			"type": "letters",
			"attributes": map[string]any{
				"file_original_name": originalName,
				"file_url":           uploadURL,
				"file_url_signature": signature,
				"address_position":   attributes["address_position"],
				"auto_send":          attributes["auto_send"],
			},
		},
	}
	if value, ok := attributes["delivery_product"]; ok {
		payload["data"].(map[string]any)["attributes"].(map[string]any)["delivery_product"] = value
	}
	if value, ok := attributes["print_mode"]; ok {
		payload["data"].(map[string]any)["attributes"].(map[string]any)["print_mode"] = value
	}
	if value, ok := attributes["print_spectrum"]; ok {
		payload["data"].(map[string]any)["attributes"].(map[string]any)["print_spectrum"] = value
	}
	if value, ok := attributes["meta_data"]; ok {
		payload["data"].(map[string]any)["attributes"].(map[string]any)["meta_data"] = value
	}

	if ctx.global.verbose && !ctx.global.quiet {
		fmt.Fprintln(os.Stderr, "creating letter...")
	}
	resp, _, err := client.CreateLetter(ctx.settings.OrganisationID, payload, *idempotencyKey)
	if err != nil {
		printError(err.Error(), 0, "")
		return 1
	}
	if ctx.global.jsonOutput {
		return emitJSON(resp)
	}
	printLetterSummary(resp)
	return 0
}

func handleLettersSend(ctx appContext, args []string) int {
	if ctx.settings.OrganisationID == "" {
		printError("organisation id required", 0, "")
		return 2
	}
	fs := flag.NewFlagSet("letters send", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	deliveryProduct := fs.String("delivery-product", "", "Delivery product")
	printMode := fs.String("print-mode", "", "Print mode")
	printSpectrum := fs.String("print-spectrum", "", "Print spectrum")
	metaJSON := fs.String("meta-json", "", "Meta data JSON string or @path")
	metaFile := fs.String("meta-file", "", "Meta data JSON file path")
	idempotencyKey := fs.String("idempotency-key", "", "Idempotency key for send request")
	help := fs.Bool("help", false, "show help")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *help {
		fmt.Println("Usage: pingen-cli letters send <letter_id> --delivery-product <fast|cheap|bulk|premium|registered> --print-mode <simplex|duplex> --print-spectrum <color|grayscale> [--meta-json ...|--meta-file ...]")
		return 0
	}
	remaining := fs.Args()
	if len(remaining) == 0 {
		printError("letter id required", 0, "")
		return 2
	}
	letterID := remaining[0]
	if *deliveryProduct == "" || *printMode == "" || *printSpectrum == "" {
		printError("delivery-product, print-mode, and print-spectrum are required", 0, "")
		return 2
	}
	if !isAllowed(*deliveryProduct, []string{"fast", "cheap", "bulk", "premium", "registered"}) {
		printError("invalid delivery-product", 0, "")
		return 2
	}
	if !isAllowed(*printMode, []string{"simplex", "duplex"}) {
		printError("invalid print-mode", 0, "")
		return 2
	}
	if !isAllowed(*printSpectrum, []string{"color", "grayscale"}) {
		printError("invalid print-spectrum", 0, "")
		return 2
	}
	metaData, err := loadJSONInput(*metaJSON, *metaFile)
	if err != nil {
		printError(err.Error(), 0, "")
		return 2
	}
	attributes := map[string]any{
		"delivery_product": *deliveryProduct,
		"print_mode":       *printMode,
		"print_spectrum":   *printSpectrum,
	}
	if metaData != nil {
		attributes["meta_data"] = metaData
	}

	if ctx.global.dryRun {
		payload := map[string]any{
			"action":          "letters.send",
			"organisation_id": ctx.settings.OrganisationID,
			"letter_id":       letterID,
			"attributes":      attributes,
		}
		return emitJSON(payload)
	}

	token, err := ensureAccessToken(&ctx)
	if err != nil {
		printError(err.Error(), 0, "")
		return 1
	}
	client := pingen.Client{
		APIBase:     ctx.settings.APIBase,
		AccessToken: token,
		Timeout:     time.Duration(ctx.global.timeout) * time.Second,
	}
	payload := map[string]any{
		"data": map[string]any{
			"id":         letterID,
			"type":       "letters",
			"attributes": attributes,
		},
	}
	resp, _, err := client.SendLetter(ctx.settings.OrganisationID, letterID, payload, *idempotencyKey)
	if err != nil {
		printError(err.Error(), 0, "")
		return 1
	}
	if ctx.global.jsonOutput {
		return emitJSON(resp)
	}
	printLetterSummary(resp)
	return 0
}

func ensureAccessToken(ctx *appContext) (string, error) {
	if ctx.settings.AccessToken != "" {
		if ctx.settings.AccessTokenExpiresAt == 0 {
			return ctx.settings.AccessToken, nil
		}
		if time.Now().Unix() < ctx.settings.AccessTokenExpiresAt-30 {
			return ctx.settings.AccessToken, nil
		}
	}
	if ctx.settings.ClientID == "" || ctx.settings.ClientSecret == "" {
		return "", fmt.Errorf("access token required (use --access-token or auth token)")
	}
	client := pingen.Client{
		APIBase:      ctx.settings.APIBase,
		IdentityBase: ctx.settings.IdentityBase,
		Timeout:      time.Duration(ctx.global.timeout) * time.Second,
	}
	payload, _, err := client.GetToken(ctx.settings.ClientID, ctx.settings.ClientSecret, defaultScope)
	if err != nil {
		return "", err
	}
	token, ok := payload["access_token"].(string)
	if !ok || token == "" {
		return "", fmt.Errorf("access token missing in response")
	}
	ctx.settings.AccessToken = token
	if ctx.configLoaded {
		cfg, _, _ := pingen.LoadConfig(ctx.configPath)
		cfg.AccessToken = token
		if expires, ok := payload["expires_in"].(float64); ok {
			cfg.AccessTokenExpiresAt = time.Now().Add(time.Duration(int64(expires)) * time.Second).Unix()
		}
		_ = pingen.SaveConfig(ctx.configPath, cfg)
	}
	return token, nil
}

func buildListParams(page, limit int, sort, filter, query, include, fields, resource string) map[string]string {
	params := map[string]string{}
	if page > 0 {
		params["page[number]"] = fmt.Sprintf("%d", page)
	}
	if limit > 0 {
		params["page[limit]"] = fmt.Sprintf("%d", limit)
	}
	if sort != "" {
		params["sort"] = sort
	}
	if filter != "" {
		if strings.HasPrefix(filter, "@") {
			content, err := os.ReadFile(strings.TrimPrefix(filter, "@"))
			if err == nil {
				filter = strings.TrimSpace(string(content))
			}
		}
		params["filter"] = filter
	}
	if query != "" {
		params["q"] = query
	}
	if include != "" {
		params["include"] = include
	}
	if fields != "" {
		params[fmt.Sprintf("fields[%s]", resource)] = fields
	}
	return params
}

func loadJSONInput(metaJSON, metaFile string) (map[string]any, error) {
	if metaJSON != "" && metaFile != "" {
		return nil, fmt.Errorf("use either --meta-json or --meta-file")
	}
	if metaFile != "" {
		content, err := os.ReadFile(metaFile)
		if err != nil {
			return nil, err
		}
		return parseJSONObject(content)
	}
	if metaJSON != "" {
		if strings.HasPrefix(metaJSON, "@") {
			content, err := os.ReadFile(strings.TrimPrefix(metaJSON, "@"))
			if err != nil {
				return nil, err
			}
			return parseJSONObject(content)
		}
		return parseJSONObject([]byte(metaJSON))
	}
	return nil, nil
}

func parseJSONObject(content []byte) (map[string]any, error) {
	var parsed map[string]any
	if err := json.Unmarshal(content, &parsed); err != nil {
		return nil, fmt.Errorf("invalid JSON payload")
	}
	return parsed, nil
}

func emitJSON(payload any) int {
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		printError("failed to encode json", 0, "")
		return 1
	}
	fmt.Println(string(encoded))
	return 0
}

func printLetterSummary(payload map[string]any) {
	data, ok := payload["data"].(map[string]any)
	if !ok {
		fmt.Println(payload)
		return
	}
	attrs, _ := data["attributes"].(map[string]any)
	fmt.Printf("%s\t%s\t%s\n", stringValue(data["id"]), stringValue(attrs["status"]), stringValue(attrs["file_original_name"]))
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case float64:
		return fmt.Sprintf("%.0f", v)
	default:
		return fmt.Sprintf("%v", value)
	}
}

func isAllowed(value string, allowed []string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func printError(message string, status int, requestID string) {
	parts := []string{message}
	if status != 0 {
		parts = append(parts, fmt.Sprintf("(HTTP %d)", status))
	}
	if requestID != "" {
		parts = append(parts, fmt.Sprintf("request_id=%s", requestID))
	}
	fmt.Fprintln(os.Stderr, strings.Join(parts, " "))
}
