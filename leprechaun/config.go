package leprechaun

import (
	"encoding/json"
	"errors"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/Tkanos/gonfig"
)

func init() {
	var i int32
	for i = 0; i <= 60; i++ {
		DefaultSnoozeTimes = append(DefaultSnoozeTimes, i)
	}
}

var (
	apiKeyID                  = flag.String("api-key-id", "89p3njup22kr", "Your Luno API key ID (*required)")
	apiKeySecret              = flag.String("api-key-secret", "awPiuPhKw9AWFR4K95AHwI_kchmMMhJ257fkZ-HJa6o", "Your Luno API key secret (*required)")
	assetsToTrade             = flag.String("assets", "xrp", `Specify assets you want Leprechaun to trade for you. Use the three-letter code of each asset seperated by a "+". e.g. To trade bitcoin and ripple coin, use "btc+xrp". Note that you must already have created a luno wallet for each asset you want to trade.`)
	purchaseUnit              = flag.Float64("purchase-unit", 600, "Specify how much you want to spend for each of Leprechaun's purchase")
	profitMargin              = flag.Float64("profit-margin", 3.0, "Minimum profit margin at which to sell assets. Refer to the help file for more information. Default is 1%")
	verbose                   = flag.Bool("verbose", true, `Setting -verbose to "true" prints the bot's output to the command line (screen). Set it to "false" to prevent this behaviour. Note that some messages will still be written to the screen. The bot's output messages are always written to a log file anyway.`)
	exitIfNoClientInitialized = flag.Bool("exit-on-init-error", false, `Setting the "exit-on-init-error" flag to true causes Leprechaun to exit immediately if it cannot connect to the exhange on startup (Ususally due to a bad internet connection). Setting it to false will cause Leprechaun to wait for some time before trying again and again. This can be useful if the user intends to let the bot run for long periods without supervision.`)
)

var (
	keyIDValue     string = "LUNO_API_KEY_ID"
	keySecretValue string = "LUNO_API_KEY_SECRET"
)

// TradeSettings defines trading specific parameteres.
type TradeSettings struct {
	ProfitMargin float64
	Shortsell    bool
	ShortTrade   struct {
		StopLoss           bool
		StopLossPercentage float64
	}
	LongTrade struct {
		StopLoss           bool
		StopLossPercentage float64
	}
	AnalysisPlugin struct {
		Name string
	}
}

// ConfigField represents a single field that can be marked to indicate its value has been changed
type ConfigField struct {
	Value   interface{}
	Updated bool
}

// Update changes the val of a config and marks it as updated
func (field *ConfigField) Update(val ...interface{}) {
	field.Value = val
	field.Updated = true
}

// Configuration object holds settings for Leprechaun.
type Configuration struct {
	Name                 string
	SupportedAssets      []string
	ExitOnInitFailed     bool
	APIKeyID             string
	APIKeySecret         string
	PurchaseUnit         float64
	AssetsToTrade        []string
	EmailAddress         string
	ProfitMargin         float64
	LedgerDatabase       string
	SnoozeTimes          []int32
	SnoozePeriod         int32
	Verbose              bool
	Debug                bool
	AdjustedPurchaseUnit float64
	Android              bool
	CurrencyCode         string
	CurrencyName         string
	RandomSnooze         bool
	AppDir               string
	DataDir              string
	LogDir               string
	keyStore             string
	configFile           string
	// TradingMode          TradeMode
	Trade TradeSettings
}

// ErrNoSavedSettings is returned by the load settigs function when it can't find any saved settings on file.
var ErrNoSavedSettings = errors.New("could not find any saved settings")

// Default vars
var (
	DefaultSnoozeTimes     []int32
	DefaultSupportedAssets = []string{"XBT", "ETH", "XRP", "LTC"}
	DefaultCurrencyName    = "Naira"
	DefaultCurrencyCode    = "NGN"
)

// DefaultSettings updates the Configuration struct to their default values.
func (c *Configuration) DefaultSettings(appDir string) error {
	conf := &Configuration{
		Name:            os.Getenv("USERPROFILE"),
		SupportedAssets: []string{"XBT", "ETH", "XRP", "LTC"},
		CurrencyCode:    "NGN", CurrencyName: "Naira",

		// TODO; EXPORT KEY ID AND SECRET TO ENV VARS FOR SECURITY
		ExitOnInitFailed: false, APIKeyID: "",
		APIKeySecret: "", PurchaseUnit: 10000,
		AssetsToTrade: []string{"XBT", "ETH", "XRP", "LTC"},
		ProfitMargin:  3 / 100.0,
		SnoozeTimes:   DefaultSnoozeTimes,
		RandomSnooze:  true,
		SnoozePeriod:  5,
		Verbose:       true,
		Debug:         false,
	}

	err := c.Update(conf, true)
	if err != nil {
		return err
	}
	if appDir != "" {
		c.SetAppDir(appDir)
	} else {
		return errors.New("app dir is not provided")
	}
	err = c.Save()
	if err != nil {
		log.Printf("Save err: %v", err)
		return err
	}
	return nil
}

// TestConfig is my custom settings for testing purposes.
func (c *Configuration) TestConfig(appDir string) error {
	flag.Parse()
	c.APIKeyID, c.APIKeySecret = *apiKeyID, *apiKeySecret
	c.ExitOnInitFailed = *exitIfNoClientInitialized
	c.ProfitMargin, c.PurchaseUnit = *profitMargin/100, *purchaseUnit
	c.CurrencyCode, c.CurrencyName = "NGN", "Naira"
	c.AssetsToTrade = []string{"XRP"}
	c.SupportedAssets = []string{"XBT", "ETH", "XRP", "LTC"}
	c.Name = os.Getenv("USERPROFILE")
	c.SnoozeTimes = []int32{1, 2, 3, 5, 7, 9, 11, 13, 15, 21, 25, 30}
	c.RandomSnooze = true
	c.SnoozePeriod = 5
	c.Verbose = true
	c.Debug = true
	if appDir != "" {
		c.SetAppDir(appDir)
	} else {
		return errors.New("app dir is not provided")
	}
	err := c.Save()
	if err != nil {
		return err
	}
	return nil
}

// Save the config struct to a json file.
func (c *Configuration) Save() error {
	dir := filepath.Dir(c.configFile)
	if !exists(dir) {
		// create folder first.
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Println("Could not create folder: ", dir)
			return err
		}
	}
	f, err := os.OpenFile(c.configFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	defer f.Close()
	if err != nil {
		log.Printf("%v", err)
		return err
	}
	// Create a copy of the `Configuration` object for Saving.
	conf := c
	e := json.NewEncoder(f).Encode(conf)
	if e != nil {
		log.Printf("Json decode error in c.Save() :%v", e)
		return err
	}
	return nil
}

// Update the config struct with user defined values and disregard invalid values
func (c *Configuration) Update(copy *Configuration, isDefault bool) (err error) {
	if copy.APIKeyID != "" || isDefault {
		c.APIKeyID = copy.APIKeyID
	}
	if copy.APIKeySecret != "" || isDefault {
		c.APIKeySecret = copy.APIKeySecret
	}
	if copy.PurchaseUnit > 0 || isDefault {
		c.PurchaseUnit = copy.PurchaseUnit
	}
	if copy.ProfitMargin > 0 || isDefault {
		c.ProfitMargin = copy.ProfitMargin
	}
	if copy.EmailAddress != "" || isDefault {
		c.EmailAddress = copy.EmailAddress
	}
	if len(copy.AssetsToTrade) > 0 || isDefault {
		c.AssetsToTrade = copy.AssetsToTrade
	}
	// for val, changed := range c{
	// 	if changed{
	// 		copy.Value = val
	// 	}
	// }

	c.RandomSnooze, c.SnoozePeriod = copy.RandomSnooze, copy.SnoozePeriod
	c.RandomSnooze = copy.RandomSnooze
	c.SupportedAssets = DefaultSupportedAssets
	c.SnoozeTimes, c.CurrencyName = DefaultSnoozeTimes, DefaultCurrencyName
	c.CurrencyCode, c.Verbose = DefaultCurrencyCode, copy.Verbose
	c.keyStore, c.ExitOnInitFailed = copy.keyStore, copy.ExitOnInitFailed
	if copy.AppDir != "" && !isDefault {
		c.SetAppDir(filepath.Dir(copy.AppDir))
	}
	return nil
}

// LoadConfig returns previously saved settings from file. If settings have not been saved it returns an error.
func (c *Configuration) LoadConfig(appDir string) (err error) {
	if c.AppDir == "" && appDir != "" {
		c.SetAppDir(appDir)
	}
	if !exists(c.configFile) {
		// No settings were saved. usually happens the first time the app is run in a new location
		return ErrNoSavedSettings
	}
	f, err := os.OpenFile(c.configFile, os.O_RDWR, 0644)
	gonfig.GetConf(c.configFile, &c)
	defer f.Close()
	if err != nil {
		return err
	}
	// Load filenames into `files`
	conf := &Configuration{}
	err = json.NewDecoder(f).Decode(&conf)
	if err != nil {
		return err
	}
	err = c.Update(conf, false)
	if err != nil {
		return err
	}
	return nil

}

// ExportAPIVars sets the api key id and key secret environment variables
func (c *Configuration) ExportAPIVars(keyID, keySecret string) (err error) {
	// Put the keys into an env var while app is running
	// Store them in an sqlite3 db after done

	os.Setenv(keyIDValue, keyID)
	err = os.Setenv(keySecretValue, keySecret)

	// Should be securely stored.
	// Consider option to hide key fields after user has succesfuly synced system.
	// Also provide option to restore default settings.
	return
}

// ImportAPIVars gets the values of the key id and key secret environment varaibles.
func (c *Configuration) ImportAPIVars() (keyID, keySecret string, err error) {
	// retrive keys from environment variables
	keyID = os.Getenv(keyIDValue)
	keySecret = os.Getenv(keySecretValue)
	return
}

// SetAppDir uses the platform-aware values from `gioui.org/app`
func (c *Configuration) SetAppDir(dir string) {
	c.AppDir = filepath.Join(dir, "Leprechaun")
	c.DataDir = filepath.Join(c.AppDir, "data")
	c.keyStore = filepath.Join(c.DataDir, "keystore.db")
	c.LedgerDatabase = filepath.Join(c.DataDir, "ledger.db")
	c.configFile = filepath.Join(c.DataDir, "config.json")
}
