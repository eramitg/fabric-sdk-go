/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package msp

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/cryptosuite/bccsp/sw"
	kvs "github.com/hyperledger/fabric-sdk-go/pkg/fab/keyvaluestore"
	mspapi "github.com/hyperledger/fabric-sdk-go/pkg/msp/api"
	"github.com/hyperledger/fabric-sdk-go/pkg/msp/mocks"
)

const (
	org1                        = "Org1"
	caServerURLListen           = "http://127.0.0.1:0"
	dummyUserStorePath          = "/tmp/userstore"
	fullConfigPath              = "testdata/config_test.yaml"
	wrongURLConfigPath          = "testdata/config_wrong_url.yaml"
	noCAConfigPath              = "testdata/config_no_ca.yaml"
	embeddedRegistrarConfigPath = "testdata/config_embedded_registrar.yaml"
	noRegistrarConfigPath       = "testdata/config_no_registrar.yaml"
)

var caServerURL string

type textFixture struct {
	config          core.Config
	cryptoSuite     core.CryptoSuite
	userStore       msp.UserStore
	identityManager *IdentityManager
	caClient        mspapi.CAClient
}

var caServer = &mocks.MockFabricCAServer{}

func (f *textFixture) setup(configPath string) {

	if configPath == "" {
		configPath = fullConfigPath
	}

	var lis net.Listener
	var err error
	if !caServer.Running() {
		lis, err = net.Listen("tcp", strings.TrimPrefix(caServerURLListen, "http://"))
		if err != nil {
			panic(fmt.Sprintf("Error starting CA Server %s", err))
		}

		caServerURL = "http://" + lis.Addr().String()
	}

	cfgRaw := readConfigWithReplacement(configPath, "http://localhost:8050", caServerURL)
	f.config, err = config.FromRaw(cfgRaw, "yaml")()
	if err != nil {
		panic(fmt.Sprintf("Failed to read config: %v", err))
	}

	// Delete all private keys from the crypto suite store
	// and users from the user store
	cleanup(f.config.KeyStorePath())
	cleanup(f.config.CredentialStorePath())

	f.cryptoSuite, err = sw.GetSuiteByConfig(f.config)
	if f.cryptoSuite == nil {
		panic(fmt.Sprintf("Failed initialize cryptoSuite: %v", err))
	}

	if f.config.CredentialStorePath() != "" {
		f.userStore, err = NewCertFileUserStore(f.config.CredentialStorePath())
		if err != nil {
			panic(fmt.Sprintf("creating a user store failed: %v", err))
		}
	}
	f.userStore = userStoreFromConfig(nil, f.config)

	f.identityManager, err = NewIdentityManager("org1", f.userStore, f.cryptoSuite, f.config)
	if err != nil {
		panic(fmt.Sprintf("manager.NewManager returned error: %v", err))
	}

	f.caClient, err = NewCAClient(org1, f.identityManager, f.userStore, f.cryptoSuite, f.config)
	if err != nil {
		panic(fmt.Sprintf("NewCAClient returned error: %v", err))
	}

	// Start Http Server if it's not running
	if !caServer.Running() {
		caServer.Start(lis, f.cryptoSuite)
	}
}

func (f *textFixture) close() {
	cleanup(f.config.CredentialStorePath())
	cleanup(f.config.KeyStorePath())
}

// readCert Reads a random cert for testing
func readCert(t *testing.T) []byte {
	cert, err := ioutil.ReadFile("testdata/root.pem")
	if err != nil {
		t.Fatalf("Error reading cert: %s", err.Error())
	}
	return cert
}

func readConfigWithReplacement(path string, origURL, newURL string) []byte {
	cfgRaw, err := ioutil.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("Failed to read config [%s]", err))
	}

	updatedCfg := strings.Replace(string(cfgRaw), origURL, newURL, -1)
	return []byte(updatedCfg)
}

func cleanup(storePath string) {
	err := os.RemoveAll(storePath)
	if err != nil {
		panic(fmt.Sprintf("Failed to remove dir %s: %v\n", storePath, err))
	}
}

func cleanupTestPath(t *testing.T, storePath string) {
	err := os.RemoveAll(storePath)
	if err != nil {
		t.Fatalf("Cleaning up directory '%s' failed: %v", storePath, err)
	}
}

func mspIDByOrgName(t *testing.T, c core.Config, orgName string) string {
	netConfig, err := c.NetworkConfig()
	if err != nil {
		t.Fatalf("network config retrieval failed: %v", err)
	}

	// viper keys are case insensitive
	orgConfig, ok := netConfig.Organizations[strings.ToLower(orgName)]
	if !ok {
		t.Fatalf("org config retrieval failed: %v", err)
	}
	return orgConfig.MSPID
}

func userStoreFromConfig(t *testing.T, config core.Config) msp.UserStore {
	stateStore, err := kvs.New(&kvs.FileKeyValueStoreOptions{Path: config.CredentialStorePath()})
	if err != nil {
		t.Fatalf("CreateNewFileKeyValueStore failed: %v", err)
	}
	userStore, err := NewCertFileUserStore1(stateStore)
	if err != nil {
		t.Fatalf("CreateNewFileKeyValueStore failed: %v", err)
	}
	return userStore
}
