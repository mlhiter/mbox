package cli

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const currentClientAPIVersion = "v1alpha1"

type apiInfo struct {
	Name          string   `json:"name"`
	APIVersion    string   `json:"apiVersion"`
	ServerVersion string   `json:"serverVersion"`
	Capabilities  []string `json:"capabilities"`
	Compatibility struct {
		MinimumCLIAPIVersion string `json:"minimumCliApiVersion"`
		MinimumSDKAPIVersion string `json:"minimumSdkApiVersion"`
	} `json:"compatibility"`
}

type CompatibilityCheckResult struct {
	OK                   bool     `json:"ok"`
	Client               string   `json:"client"`
	ClientAPIVersion     string   `json:"clientApiVersion"`
	ServerAPIVersion     string   `json:"serverApiVersion"`
	MinimumAPIVersion    string   `json:"minimumApiVersion"`
	RequiredCapabilities []string `json:"requiredCapabilities"`
	MissingCapabilities  []string `json:"missingCapabilities"`
	Message              string   `json:"message"`
}

func CheckCLICompatibility(info apiInfo, clientAPIVersion string, requiredCapabilities []string) CompatibilityCheckResult {
	if strings.TrimSpace(clientAPIVersion) == "" {
		clientAPIVersion = currentClientAPIVersion
	}
	minimum := info.Compatibility.MinimumCLIAPIVersion
	normalizedRequiredCapabilities := normalizeCapabilities(requiredCapabilities)
	missingCapabilities := findMissingCapabilities(info.Capabilities, normalizedRequiredCapabilities)
	clientVersion, clientOK := parseAPIVersion(clientAPIVersion)
	minimumVersion, minimumOK := parseAPIVersion(minimum)
	serverVersion, serverOK := parseAPIVersion(info.APIVersion)
	versionOK := clientOK && minimumOK && serverOK &&
		clientVersion.family == minimumVersion.family &&
		serverVersion.family == minimumVersion.family &&
		clientVersion.order >= minimumVersion.order
	message := fmt.Sprintf("mbox cli API %s is compatible with server %s", clientAPIVersion, info.APIVersion)
	if !versionOK {
		message = fmt.Sprintf(
			"mbox cli API %s is not compatible with server %s; server requires %s",
			clientAPIVersion,
			info.APIVersion,
			unknownIfEmpty(minimum),
		)
	} else if len(missingCapabilities) > 0 {
		message = fmt.Sprintf("mbox server %s is missing required capabilities: %s", info.APIVersion, strings.Join(missingCapabilities, ", "))
	}
	return CompatibilityCheckResult{
		OK:                   versionOK && len(missingCapabilities) == 0,
		Client:               "cli",
		ClientAPIVersion:     clientAPIVersion,
		ServerAPIVersion:     info.APIVersion,
		MinimumAPIVersion:    minimum,
		RequiredCapabilities: normalizedRequiredCapabilities,
		MissingCapabilities:  missingCapabilities,
		Message:              message,
	}
}

func normalizeCapabilities(capabilities []string) []string {
	out := []string{}
	seen := map[string]struct{}{}
	for _, capability := range capabilities {
		clean := strings.TrimSpace(capability)
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	return out
}

func findMissingCapabilities(available []string, required []string) []string {
	availableSet := map[string]struct{}{}
	for _, capability := range normalizeCapabilities(available) {
		availableSet[capability] = struct{}{}
	}
	missing := []string{}
	for _, capability := range required {
		if _, ok := availableSet[capability]; !ok {
			missing = append(missing, capability)
		}
	}
	return missing
}

type apiVersion struct {
	family string
	order  int
}

var apiVersionPattern = regexp.MustCompile(`^v([0-9]+)(?:(alpha|beta)([0-9]+))?$`)

func parseAPIVersion(raw string) (apiVersion, bool) {
	match := apiVersionPattern.FindStringSubmatch(strings.TrimSpace(raw))
	if match == nil {
		return apiVersion{}, false
	}
	major, err := strconv.Atoi(match[1])
	if err != nil {
		return apiVersion{}, false
	}
	stage := match[2]
	stageNumber := 0
	if match[3] != "" {
		stageNumber, err = strconv.Atoi(match[3])
		if err != nil {
			return apiVersion{}, false
		}
	}
	stageRank := 2
	switch stage {
	case "alpha":
		stageRank = 0
	case "beta":
		stageRank = 1
	}
	return apiVersion{
		family: fmt.Sprintf("v%d", major),
		order:  major*1_000_000 + stageRank*1_000 + stageNumber,
	}, true
}

func unknownIfEmpty(value string) string {
	if strings.TrimSpace(value) == "" {
		return "an unknown minimum API version"
	}
	return value
}
