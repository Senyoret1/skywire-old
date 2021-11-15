package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/skycoin/dmsg/cipher"
	coinCipher "github.com/skycoin/skycoin/src/cipher"
	"github.com/skycoin/skycoin/src/util/logging"
	"github.com/spf13/cobra"

	"github.com/skycoin/skywire/pkg/skyenv"
	"github.com/skycoin/skywire/pkg/visor/visorconfig"
)

func init() {
	RootCmd.AddCommand(genConfigCmd)
}

var (
	sk                 cipher.SecKey
	output             string
	replace            bool
	replaceHypervisors bool
	testEnv            bool
	packageConfig      bool
	skybianConfig      bool
	hypervisor         bool
	hypervisorPKs      string
	dmsgHttp           bool
)

func init() {
	genConfigCmd.Flags().Var(&sk, "sk", "if unspecified, a random key pair will be generated.\n")
	genConfigCmd.Flags().StringVarP(&output, "output", "o", "skywire-config.json", "path of output config file.")
	genConfigCmd.Flags().BoolVarP(&replace, "replace", "r", false, "rewrite existing config (retains keys).")
	genConfigCmd.Flags().BoolVarP(&replaceHypervisors, "use-old-hypervisors", "x", false, "use old hypervisors keys.")
	genConfigCmd.Flags().BoolVarP(&packageConfig, "package", "p", false, "use defaults for package-based installations in /opt/skywire")
	genConfigCmd.Flags().BoolVarP(&skybianConfig, "skybian", "s", false, "use defaults paths found in skybian\n writes config to /etc/skywire-config.json")
	genConfigCmd.Flags().BoolVarP(&testEnv, "testenv", "t", false, "use test deployment service.")
	genConfigCmd.Flags().BoolVarP(&hypervisor, "is-hypervisor", "i", false, "generate a hypervisor configuration.")
	genConfigCmd.Flags().StringVar(&hypervisorPKs, "hypervisor-pks", "", "public keys of hypervisors that should be added to this visor")
	genConfigCmd.Flags().BoolVarP(&dmsgHttp, "dmsghttp", "d", false, "connect to Skywire Services via dmsg")
}

var genConfigCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generates a config file",
	PreRun: func(_ *cobra.Command, _ []string) {
		var err error
		if output, err = filepath.Abs(output); err != nil {
			logger.WithError(err).Fatal("Invalid output provided.")
		}
	},
	Run: func(_ *cobra.Command, _ []string) {
		mLog := logging.NewMasterLogger()
		mLog.SetLevel(logrus.InfoLevel)

		//Fail on -pst combination
		if (packageConfig && skybianConfig) || (packageConfig && testEnv) || (skybianConfig && testEnv) {
			logger.Fatal("Failed to create config: use of mutually exclusive flags")
		}

		//set output for package and skybian configs
		if packageConfig {
			configName := "skywire-config.json"
			output = filepath.Join(skyenv.PackageSkywirePath(), configName)
		}

		if skybianConfig {
			output = "/etc/skywire-config.json"
		}

		// Read in old config (if any) and obtain old secret key.
		// Otherwise, we generate a new random secret key.
		var sk cipher.SecKey
		if oldConf, ok := readOldConfig(mLog, output, replace); !ok {
			_, sk = cipher.GenerateKeyPair()
		} else {
			sk = oldConf.SK
		}

		// Determine config type to generate.
		var genConf func(log *logging.MasterLogger, confPath string, sk *cipher.SecKey, hypervisor bool) (*visorconfig.V1, error)

		//  default paths for different installations
		if packageConfig {
			genConf = visorconfig.MakePackageConfig
		} else if skybianConfig {
			genConf = visorconfig.MakeDefaultConfig
		} else if testEnv {
			genConf = visorconfig.MakeTestConfig
		} else {
			genConf = visorconfig.MakeDefaultConfig
		}

		// Generate config.
		conf, err := genConf(mLog, output, &sk, hypervisor)
		if err != nil {
			logger.WithError(err).Fatal("Failed to create config.")
		}

		// Use local servers
		if dmsgHttp {
			var localServersData localServers
			serversListJSON, err := ioutil.ReadFile("localServers.json")
			if err != nil {
				logger.WithError(err).Fatal("Failed to read servers.json file.")
			}
			err = json.Unmarshal(serversListJSON, &localServersData)
			if err != nil {
				logger.WithError(err).Fatal("Error during parsing servers list")
			}
			if testEnv {
				conf.Dmsg.Servers = localServersData.Test.DMSGServers
				conf.Dmsg.Discovery = localServersData.Test.DMSGDiscovery
				conf.Transport.AddressResolver = localServersData.Test.AddressResolver
				conf.Transport.Discovery = localServersData.Test.TransportDiscovery
				conf.UptimeTracker.Addr = localServersData.Test.UptimeTracker
				conf.Routing.RouteFinder = localServersData.Test.RouteFinder
				conf.Launcher.ServiceDisc = localServersData.Test.ServiceDiscovery
			} else {
				conf.Dmsg.Servers = localServersData.Prod.DMSGServers
				conf.Dmsg.Discovery = localServersData.Prod.DMSGDiscovery
				conf.Transport.AddressResolver = localServersData.Prod.AddressResolver
				conf.Transport.Discovery = localServersData.Prod.TransportDiscovery
				conf.UptimeTracker.Addr = localServersData.Prod.UptimeTracker
				conf.Routing.RouteFinder = localServersData.Prod.RouteFinder
				conf.Launcher.ServiceDisc = localServersData.Prod.ServiceDiscovery
			}
		}

		// Read in old config (if any) and obtain old hypervisors.
		if replaceHypervisors {
			if oldConf, ok := readOldConfig(mLog, output, true); ok {
				conf.Hypervisors = oldConf.Hypervisors
			}
		}

		if hypervisorPKs != "" {
			keys := strings.Split(hypervisorPKs, ",")
			for _, key := range keys {
				keyParsed, err := coinCipher.PubKeyFromHex(strings.TrimSpace(key))
				if err != nil {
					logger.WithError(err).Fatalf("Failed to parse hypervisor private key: %s.", key)
				}
				conf.Hypervisors = append(conf.Hypervisors, cipher.PubKey(keyParsed))
			}
		}

		// Save config to file.
		if err := conf.Flush(); err != nil {
			logger.WithError(err).Fatal("Failed to flush config to file.")
		}

		// Print results.
		j, err := json.MarshalIndent(conf, "", "\t")
		if err != nil {
			logger.WithError(err).Fatal("An unexpected error occurred. Please contact a developer.")
		}
		logger.Infof("Updated file '%s' to: %s", output, j)
	},
}

func readOldConfig(log *logging.MasterLogger, confPath string, replace bool) (*visorconfig.V1, bool) {
	raw, err := ioutil.ReadFile(confPath) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false
		}
		logger.WithError(err).Fatal("Unexpected error occurred when attempting to read old config.")
	}

	if !replace {
		logger.Fatal("Config file already exists. Specify the 'replace, r' flag to replace this.")
	}

	conf, err := visorconfig.Parse(log, confPath, raw)
	if err != nil {
		logger.WithError(err).Fatal("Failed to parse old config file.")
	}

	return conf, true
}

type localServers struct {
	Test localServersData `json:"test"`
	Prod localServersData `json:"prod"`
}
type localServersData struct {
	DMSGServers        []string `json:"dmsg_servers"`
	DMSGDiscovery      string   `json:"dmsg_discovery"`
	TransportDiscovery string   `json:"transport_discovery"`
	AddressResolver    string   `json:"address_resolver"`
	RouteFinder        string   `json:"route_finder"`
	UptimeTracker      string   `json:"uptime_tracker"`
	ServiceDiscovery   string   `json:"service_discovery"`
}
