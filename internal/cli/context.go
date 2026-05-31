package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type contextConfigFile struct {
	CurrentContext string                  `json:"currentContext"`
	Contexts       map[string]contextEntry `json:"contexts"`
}

type contextEntry struct {
	APIURL      string `json:"apiUrl"`
	Token       string `json:"token"`
	TokenEnv    string `json:"tokenEnv"`
	AuditActor  string `json:"auditActor"`
	AuditSource string `json:"auditSource"`
}

type contextSelection struct {
	Name        string `json:"name,omitempty"`
	ConfigPath  string `json:"configPath,omitempty"`
	APIURL      string `json:"apiUrl"`
	HasToken    bool   `json:"hasToken"`
	AuditActor  string `json:"auditActor,omitempty"`
	AuditSource string `json:"auditSource,omitempty"`
}

type contextListItem struct {
	Name        string `json:"name"`
	Current     bool   `json:"current"`
	APIURL      string `json:"apiUrl,omitempty"`
	HasToken    bool   `json:"hasToken"`
	AuditActor  string `json:"auditActor,omitempty"`
	AuditSource string `json:"auditSource,omitempty"`
}

type contextMutationResult struct {
	Name           string `json:"name,omitempty"`
	ConfigPath     string `json:"configPath"`
	CurrentContext string `json:"currentContext,omitempty"`
	Removed        bool   `json:"removed,omitempty"`
}

func (a *App) applyContext(config *globalConfig) error {
	contextName := strings.TrimSpace(config.Context)
	configPath, err := a.resolveConfigPath(strings.TrimSpace(config.ConfigPath))
	if err != nil {
		return err
	}
	if contextName == "" && configPath == "" {
		return nil
	}
	file, err := loadContextConfig(configPath)
	if err != nil {
		return err
	}
	if contextName == "" {
		contextName = strings.TrimSpace(file.CurrentContext)
	}
	if contextName == "" {
		return usageError("mbox context is not set and config has no currentContext")
	}
	entry, ok := file.Contexts[contextName]
	if !ok {
		return usageError(fmt.Sprintf("mbox context %q was not found in %s", contextName, configPath))
	}
	if !config.apiURLSet && strings.TrimSpace(config.APIURL) == "" {
		config.APIURL = strings.TrimSpace(entry.APIURL)
	}
	if !config.tokenSet && strings.TrimSpace(config.Token) == "" {
		config.Token = entry.resolveToken(a.getenv)
	}
	if !config.auditActorSet && strings.TrimSpace(config.AuditActor) == "" {
		config.AuditActor = strings.TrimSpace(entry.AuditActor)
	}
	if !config.auditSourceSet && strings.TrimSpace(config.AuditSource) == "" {
		config.AuditSource = strings.TrimSpace(entry.AuditSource)
	}
	return nil
}

func (a *App) runContext(ctx context.Context, config globalConfig, args []string) error {
	if len(args) < 1 {
		return usageError("usage: mbox context current|list|set|use|remove")
	}
	switch args[0] {
	case "current":
		if len(args) != 1 {
			return usageError("usage: mbox context current")
		}
		configPath, file, err := a.loadContextForRead(strings.TrimSpace(config.ConfigPath))
		if err != nil {
			return err
		}
		selectedName := strings.TrimSpace(config.Context)
		if selectedName == "" {
			selectedName = strings.TrimSpace(file.CurrentContext)
		}
		selection, err := a.currentContextSelection(config, configPath, file, selectedName)
		if err != nil {
			return err
		}
		return WriteJSON(a.streams.Stdout, selection)
	case "list":
		if len(args) != 1 {
			return usageError("usage: mbox context list")
		}
		configPath, file, err := a.loadContextForRead(strings.TrimSpace(config.ConfigPath))
		if err != nil {
			return err
		}
		selectedName := strings.TrimSpace(config.Context)
		if selectedName == "" {
			selectedName = strings.TrimSpace(file.CurrentContext)
		}
		return WriteJSON(a.streams.Stdout, a.contextList(configPath, file, selectedName))
	case "set":
		return a.setContext(ctx, config, args[1:])
	case "use":
		return a.useContext(ctx, config, args[1:])
	case "remove", "delete":
		return a.removeContext(ctx, config, args[1:])
	default:
		return usageError("usage: mbox context current|list|set|use|remove")
	}
}

func (a *App) loadContextForRead(rawPath string) (string, contextConfigFile, error) {
	configPath, err := a.resolveConfigPath(rawPath)
	if err != nil {
		return "", contextConfigFile{}, err
	}
	file := contextConfigFile{Contexts: map[string]contextEntry{}}
	if configPath != "" {
		file, err = loadContextConfig(configPath)
		if err != nil {
			return "", contextConfigFile{}, err
		}
	}
	return configPath, file, nil
}

func (a *App) loadContextForWrite(rawPath string) (string, contextConfigFile, error) {
	configPath, err := a.resolveWritableConfigPath(rawPath)
	if err != nil {
		return "", contextConfigFile{}, err
	}
	file := contextConfigFile{Contexts: map[string]contextEntry{}}
	stat, statErr := os.Stat(configPath)
	if statErr == nil {
		if stat.Size() > 0 {
			file, err = loadContextConfig(configPath)
			if err != nil {
				return "", contextConfigFile{}, err
			}
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return "", contextConfigFile{}, statErr
	}
	if file.Contexts == nil {
		file.Contexts = map[string]contextEntry{}
	}
	return configPath, file, nil
}

func (a *App) setContext(_ context.Context, config globalConfig, args []string) error {
	fs := flag.NewFlagSet("context set", flag.ContinueOnError)
	fs.SetOutput(a.streams.Stderr)
	apiURL := fs.String("api-url", "", "")
	token := fs.String("token", "", "")
	tokenEnv := fs.String("token-env", "", "")
	auditActor := fs.String("audit-actor", "", "")
	auditSource := fs.String("audit-source", "", "")
	current := fs.Bool("current", false, "")
	name := ""
	parseArgs := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name = strings.TrimSpace(args[0])
		parseArgs = args[1:]
	}
	if err := fs.Parse(parseArgs); err != nil {
		return err
	}
	if name == "" && fs.NArg() == 1 {
		name = strings.TrimSpace(fs.Arg(0))
	} else if fs.NArg() != 0 {
		return usageError("usage: mbox context set NAME --api-url URL [--token TOKEN|--token-env ENV] [--audit-actor ACTOR] [--audit-source SOURCE] [--current]")
	}
	if name == "" {
		return usageError("usage: mbox context set NAME --api-url URL [--token TOKEN|--token-env ENV] [--audit-actor ACTOR] [--audit-source SOURCE] [--current]")
	}
	apiURLValue := strings.TrimSpace(*apiURL)
	if apiURLValue == "" && config.apiURLSet {
		apiURLValue = strings.TrimSpace(config.APIURL)
	}
	tokenValue := strings.TrimSpace(*token)
	if tokenValue == "" && config.tokenSet {
		tokenValue = strings.TrimSpace(config.Token)
	}
	auditActorValue := strings.TrimSpace(*auditActor)
	if auditActorValue == "" && config.auditActorSet {
		auditActorValue = strings.TrimSpace(config.AuditActor)
	}
	auditSourceValue := strings.TrimSpace(*auditSource)
	if auditSourceValue == "" && config.auditSourceSet {
		auditSourceValue = strings.TrimSpace(config.AuditSource)
	}
	if apiURLValue == "" {
		return usageError("context set requires --api-url")
	}
	if tokenValue != "" && strings.TrimSpace(*tokenEnv) != "" {
		return usageError("context set accepts only one of --token or --token-env")
	}
	configPath, file, err := a.loadContextForWrite(strings.TrimSpace(config.ConfigPath))
	if err != nil {
		return err
	}
	file.Contexts[name] = contextEntry{
		APIURL:      apiURLValue,
		Token:       tokenValue,
		TokenEnv:    strings.TrimSpace(*tokenEnv),
		AuditActor:  auditActorValue,
		AuditSource: auditSourceValue,
	}
	if *current || strings.TrimSpace(file.CurrentContext) == "" {
		file.CurrentContext = name
	}
	if err := writeContextConfig(configPath, file); err != nil {
		return err
	}
	selection, err := a.currentContextSelection(globalConfig{}, configPath, file, name)
	if err != nil {
		return err
	}
	return WriteJSON(a.streams.Stdout, selection)
}

func (a *App) useContext(_ context.Context, config globalConfig, args []string) error {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return usageError("usage: mbox context use NAME")
	}
	name := strings.TrimSpace(args[0])
	configPath, file, err := a.loadContextForWrite(strings.TrimSpace(config.ConfigPath))
	if err != nil {
		return err
	}
	if _, ok := file.Contexts[name]; !ok {
		return usageError(fmt.Sprintf("mbox context %q was not found in %s", name, configPath))
	}
	file.CurrentContext = name
	if err := writeContextConfig(configPath, file); err != nil {
		return err
	}
	return WriteJSON(a.streams.Stdout, contextMutationResult{
		Name:           name,
		ConfigPath:     configPath,
		CurrentContext: file.CurrentContext,
	})
}

func (a *App) removeContext(_ context.Context, config globalConfig, args []string) error {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return usageError("usage: mbox context remove NAME")
	}
	name := strings.TrimSpace(args[0])
	configPath, file, err := a.loadContextForWrite(strings.TrimSpace(config.ConfigPath))
	if err != nil {
		return err
	}
	if _, ok := file.Contexts[name]; !ok {
		return usageError(fmt.Sprintf("mbox context %q was not found in %s", name, configPath))
	}
	delete(file.Contexts, name)
	if file.CurrentContext == name {
		file.CurrentContext = firstContextName(file.Contexts)
	}
	if err := writeContextConfig(configPath, file); err != nil {
		return err
	}
	return WriteJSON(a.streams.Stdout, contextMutationResult{
		Name:           name,
		ConfigPath:     configPath,
		CurrentContext: file.CurrentContext,
		Removed:        true,
	})
}

func (a *App) currentContextSelection(config globalConfig, configPath string, file contextConfigFile, selectedName string) (contextSelection, error) {
	entry := contextEntry{}
	if selectedName != "" {
		var ok bool
		entry, ok = file.Contexts[selectedName]
		if !ok && configPath != "" {
			return contextSelection{}, usageError(fmt.Sprintf("mbox context %q was not found in %s", selectedName, configPath))
		}
	}
	apiURL := strings.TrimSpace(config.APIURL)
	if !config.apiURLSet && apiURL == "" {
		apiURL = strings.TrimSpace(entry.APIURL)
	}
	if apiURL == "" {
		apiURL = defaultAPIURL
	}
	token := strings.TrimSpace(config.Token)
	if !config.tokenSet && token == "" {
		token = entry.resolveToken(a.getenv)
	}
	auditActor := strings.TrimSpace(config.AuditActor)
	if !config.auditActorSet && auditActor == "" {
		auditActor = strings.TrimSpace(entry.AuditActor)
	}
	auditSource := strings.TrimSpace(config.AuditSource)
	if !config.auditSourceSet && auditSource == "" {
		auditSource = strings.TrimSpace(entry.AuditSource)
	}
	return contextSelection{
		Name:        selectedName,
		ConfigPath:  configPath,
		APIURL:      apiURL,
		HasToken:    token != "",
		AuditActor:  auditActor,
		AuditSource: auditSource,
	}, nil
}

func (a *App) contextList(_ string, file contextConfigFile, selectedName string) map[string][]contextListItem {
	names := make([]string, 0, len(file.Contexts))
	for name := range file.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	items := make([]contextListItem, 0, len(names))
	for _, name := range names {
		entry := file.Contexts[name]
		items = append(items, contextListItem{
			Name:        name,
			Current:     name == selectedName,
			APIURL:      strings.TrimSpace(entry.APIURL),
			HasToken:    entry.resolveToken(a.getenv) != "",
			AuditActor:  strings.TrimSpace(entry.AuditActor),
			AuditSource: strings.TrimSpace(entry.AuditSource),
		})
	}
	return map[string][]contextListItem{"items": items}
}

func (a *App) resolveWritableConfigPath(path string) (string, error) {
	if strings.TrimSpace(path) != "" {
		return expandHome(strings.TrimSpace(path), a.homeDir)
	}
	home, err := a.homeDir()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(home) == "" {
		return "", usageError("home directory is unavailable; pass --config PATH")
	}
	return filepath.Join(home, ".mbox", "config.json"), nil
}

func (a *App) resolveConfigPath(path string) (string, error) {
	if path != "" {
		return expandHome(path, a.homeDir)
	}
	home, err := a.homeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", nil
	}
	candidate := filepath.Join(home, ".mbox", "config.json")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	} else if errors.Is(err, os.ErrNotExist) {
		return "", nil
	} else {
		return "", err
	}
}

func loadContextConfig(path string) (contextConfigFile, error) {
	if strings.TrimSpace(path) == "" {
		return contextConfigFile{}, usageError("mbox config path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return contextConfigFile{}, err
	}
	var file contextConfigFile
	if err := json.Unmarshal(data, &file); err != nil {
		return contextConfigFile{}, fmt.Errorf("parse mbox config %s: %w", path, err)
	}
	if file.Contexts == nil {
		file.Contexts = map[string]contextEntry{}
	}
	return file, nil
}

func writeContextConfig(path string, file contextConfigFile) error {
	if strings.TrimSpace(path) == "" {
		return usageError("mbox config path is empty")
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func firstContextName(contexts map[string]contextEntry) string {
	names := make([]string, 0, len(contexts))
	for name := range contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func (entry contextEntry) resolveToken(getenv func(string) string) string {
	if strings.TrimSpace(entry.Token) != "" {
		return strings.TrimSpace(entry.Token)
	}
	if strings.TrimSpace(entry.TokenEnv) == "" {
		return ""
	}
	return strings.TrimSpace(getenv(strings.TrimSpace(entry.TokenEnv)))
}

func expandHome(path string, homeDir func() (string, error)) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := homeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}
