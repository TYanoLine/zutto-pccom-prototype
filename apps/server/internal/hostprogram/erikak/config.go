package erikak

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	FeatureBoard         = "board"
	FeatureFile          = "file"
	FeatureMail          = "mail"
	FeatureTelegramChat  = "telegram_chat"
	FeatureJunk          = "junk"
	FeatureSettings      = "settings"
	FeatureSysopMail     = "sysop_mail"
	FeatureEnrollment    = "enrollment"
	FeatureAutoRun       = "auto_run"
	FeatureBoardMap      = "board_map"
	FeatureUnreadSearch  = "unread_search"
	FeatureAccessLog     = "access_log"
	FeatureBatchDownload = "batch_download"
	FeatureMemberList    = "member_list"
	FeatureProfile       = "profile"
)

var knownFeatures = map[string]struct{}{
	FeatureBoard: {}, FeatureFile: {}, FeatureMail: {}, FeatureTelegramChat: {},
	FeatureJunk: {}, FeatureSettings: {}, FeatureSysopMail: {}, FeatureEnrollment: {},
	FeatureAutoRun: {}, FeatureBoardMap: {}, FeatureUnreadSearch: {}, FeatureAccessLog: {},
	FeatureBatchDownload: {}, FeatureMemberList: {}, FeatureProfile: {},
}

type TransferProtocol struct {
	ID      string
	Key     string
	Label   string
	Aliases []string
}

var transferProtocolCatalog = []TransferProtocol{
	{ID: "raw", Key: "0", Label: "無手順", Aliases: []string{"RAW", "NONE", "無手順"}},
	{ID: "xmodem", Key: "1", Label: "XMODEM", Aliases: []string{"XMODEM"}},
	{ID: "xmodem_crc", Key: "2", Label: "XMODEM CRC", Aliases: []string{"XMODEM CRC", "XMODEM-CRC", "CRC"}},
	{ID: "xmodem_1k", Key: "3", Label: "XMODEM 1K", Aliases: []string{"XMODEM 1K", "XMODEM-1K", "1K"}},
	{ID: "ymodem", Key: "4", Label: "YMODEM", Aliases: []string{"YMODEM"}},
	{ID: "ymodem_g", Key: "5", Label: "YMODEM-g", Aliases: []string{"YMODEM-G", "YMODEM G"}},
	{ID: "zmodem", Key: "6", Label: "ZMODEM", Aliases: []string{"ZMODEM"}},
	{ID: "nmodem", Key: "7", Label: "NMODEM", Aliases: []string{"NMODEM"}},
}

var knownProtocols = func() map[string]struct{} {
	out := make(map[string]struct{}, len(transferProtocolCatalog))
	for _, p := range transferProtocolCatalog {
		out[p.ID] = struct{}{}
	}
	return out
}()

// Config is a modern reconstruction-time station configuration. It is not
// intended to reproduce the original Erika-K configuration-file syntax.
//
// Omitted feature/protocol keys default to enabled. This makes the historical
// runtime opt-out: station operators only need to disable capabilities that
// should not exist on that host.
type Config struct {
	Features          map[string]bool `json:"features,omitempty"`
	TransferProtocols map[string]bool `json:"transfer_protocols,omitempty"`
}

func DefaultConfig() Config {
	cfg := Config{
		Features:          make(map[string]bool, len(knownFeatures)),
		TransferProtocols: make(map[string]bool, len(knownProtocols)),
	}
	for k := range knownFeatures {
		cfg.Features[k] = true
	}
	for k := range knownProtocols {
		cfg.TransferProtocols[k] = true
	}
	return cfg
}

// ParseConfigJSON overlays a partial JSON document on top of DefaultConfig.
// Unknown keys are rejected so a misspelled "false" cannot silently enable a
// feature by falling back to the default.
func LoadConfigFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return ParseConfigJSON(data)
}

func ParseConfigJSON(data []byte) (Config, error) {
	var raw Config
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, err
	}
	for k := range raw.Features {
		if _, ok := knownFeatures[k]; !ok {
			return Config{}, fmt.Errorf("unknown Erika-K feature %q", k)
		}
	}
	for k := range raw.TransferProtocols {
		if _, ok := knownProtocols[k]; !ok {
			return Config{}, fmt.Errorf("unknown Erika-K transfer protocol %q", k)
		}
	}
	cfg := DefaultConfig()
	for k, v := range raw.Features {
		cfg.Features[k] = v
	}
	for k, v := range raw.TransferProtocols {
		cfg.TransferProtocols[k] = v
	}
	return cfg, nil
}

func (c Config) FeatureEnabled(name string) bool {
	if c.Features == nil {
		return true
	}
	v, ok := c.Features[name]
	if !ok {
		return true
	}
	return v
}

func (c Config) ProtocolEnabled(id string) bool {
	if c.TransferProtocols == nil {
		return true
	}
	v, ok := c.TransferProtocols[id]
	if !ok {
		return true
	}
	return v
}

func (c Config) EnabledTransferProtocols() []TransferProtocol {
	out := make([]TransferProtocol, 0, len(transferProtocolCatalog))
	for _, p := range transferProtocolCatalog {
		if c.ProtocolEnabled(p.ID) {
			out = append(out, p)
		}
	}
	return out
}

func findTransferProtocol(input string) (TransferProtocol, bool) {
	value := strings.TrimSpace(input)
	upper := strings.ToUpper(value)
	for _, p := range transferProtocolCatalog {
		if value == p.Key || strings.EqualFold(value, p.ID) || strings.EqualFold(value, p.Label) {
			return p, true
		}
		for _, alias := range p.Aliases {
			if upper == strings.ToUpper(alias) {
				return p, true
			}
		}
	}
	return TransferProtocol{}, false
}
