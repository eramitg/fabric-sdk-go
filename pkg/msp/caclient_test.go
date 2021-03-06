/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package msp

import (
	"testing"
	"time"

	"fmt"
	"strings"

	"github.com/golang/mock/gomock"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	mockCore "github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core/mocks"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	bccspwrapper "github.com/hyperledger/fabric-sdk-go/pkg/core/cryptosuite/bccsp/wrapper"
	"github.com/hyperledger/fabric-sdk-go/pkg/msp/api"
	"github.com/hyperledger/fabric-sdk-go/pkg/msp/mocks"
	"github.com/pkg/errors"
)

// TestEnrollAndReenroll tests enrol/reenroll scenarios
func TestEnrollAndReenroll(t *testing.T) {

	f := textFixture{}
	f.setup("")
	defer f.close()

	orgMSPID := mspIDByOrgName(t, f.config, org1)

	// Empty enrollment ID
	err := f.caClient.Enroll("", "user1")
	if err == nil {
		t.Fatalf("Enroll didn't return error")
	}

	// Empty enrollment secret
	err = f.caClient.Enroll("enrolledUsername", "")
	if err == nil {
		t.Fatalf("Enroll didn't return error")
	}

	// Successful enrollment
	enrollUsername := createRandomName()
	enrolledUserData, err := f.userStore.Load(msp.IdentityIdentifier{MSPID: orgMSPID, ID: enrollUsername})
	if err != msp.ErrUserNotFound {
		t.Fatalf("Expected to not find user in user store")
	}
	err = f.caClient.Enroll(enrollUsername, "enrollmentSecret")
	if err != nil {
		t.Fatalf("identityManager Enroll return error %v", err)
	}
	enrolledUserData, err = f.userStore.Load(msp.IdentityIdentifier{MSPID: orgMSPID, ID: enrollUsername})
	if err != nil {
		t.Fatalf("Expected to load user from user store")
	}

	// Reenroll with empty user
	err = f.caClient.Reenroll("")
	if err == nil {
		t.Fatalf("Expected error with enpty user")
	}
	if err.Error() != "user name missing" {
		t.Fatalf("Expected error user required. Got: %s", err.Error())
	}

	// Reenroll with appropriate user
	enrolledUser, err := f.identityManager.NewUser(enrolledUserData)
	if err != nil {
		t.Fatalf("newUser return error %v", err)
	}
	err = f.caClient.Reenroll(enrolledUser.Identifier().ID)
	if err != nil {
		t.Fatalf("Reenroll return error %v", err)
	}
}

// TestWrongURL tests creation of CAClient with wrong URL
func TestWrongURL(t *testing.T) {

	f := textFixture{}
	f.setup("")
	defer f.close()

	wrongURLConfigConfig, err := config.FromFile(wrongURLConfigPath)()
	if err != nil {
		panic(fmt.Sprintf("Failed to read config: %v", err))
	}

	f.caClient, err = NewCAClient(org1, f.identityManager, f.userStore, f.cryptoSuite, wrongURLConfigConfig)
	if err != nil {
		t.Fatalf("NewidentityManagerClient return error: %v", err)
	}
	err = f.caClient.Enroll("enrollmentID", "enrollmentSecret")
	if err == nil {
		t.Fatalf("Enroll didn't return error")
	}

}

// TestWrongURL tests creation of CAClient when there are no configured CAs
func TestNoConfiguredCAs(t *testing.T) {

	f := textFixture{}
	f.setup("")
	defer f.close()

	wrongURLConfigConfig, err := config.FromFile(noCAConfigPath)()
	if err != nil {
		panic(fmt.Sprintf("Failed to read config: %v", err))
	}

	_, err = NewCAClient(org1, f.identityManager, f.userStore, f.cryptoSuite, wrongURLConfigConfig)
	if err == nil || !strings.Contains(err.Error(), "no CAs configured") {
		t.Fatalf("Expected error when there are no configured CAs")
	}

}

// TestRegister tests multiple scenarios of registering a test (mocked or nil user) and their certs
func TestRegister(t *testing.T) {

	time.Sleep(2 * time.Second)

	f := textFixture{}
	f.setup("")
	defer f.close()

	// Register with nil request
	_, err := f.caClient.Register(nil)
	if err == nil {
		t.Fatalf("Expected error with nil request")
	}

	// Register without registration name parameter
	_, err = f.caClient.Register(&api.RegistrationRequest{})
	if err == nil {
		t.Fatalf("Expected error without registration name parameter")
	}

	// Register with valid request
	var attributes []api.Attribute
	attributes = append(attributes, api.Attribute{Key: "test1", Value: "test2"})
	attributes = append(attributes, api.Attribute{Key: "test2", Value: "test3"})
	secret, err := f.caClient.Register(&api.RegistrationRequest{Name: "test", Affiliation: "test", Attributes: attributes})
	if err != nil {
		t.Fatalf("identityManager Register return error %v", err)
	}
	if secret != "mockSecretValue" {
		t.Fatalf("identityManager Register return wrong value %s", secret)
	}
}

// TestEmbeddedRegistar tests registration with embedded registrar idenityt
func TestEmbeddedRegistar(t *testing.T) {

	f := textFixture{}
	f.setup(embeddedRegistrarConfigPath)
	defer f.close()

	// Register with valid request
	var attributes []api.Attribute
	attributes = append(attributes, api.Attribute{Key: "test1", Value: "test2"})
	attributes = append(attributes, api.Attribute{Key: "test2", Value: "test3"})
	secret, err := f.caClient.Register(&api.RegistrationRequest{Name: "withEmbeddedRegistrar", Affiliation: "test", Attributes: attributes})
	if err != nil {
		t.Fatalf("identityManager Register return error %v", err)
	}
	if secret != "mockSecretValue" {
		t.Fatalf("identityManager Register return wrong value %s", secret)
	}
}

// TestRegisterNoRegistrar tests registration with no configured registrar identity
func TestRegisterNoRegistrar(t *testing.T) {

	f := textFixture{}
	f.setup(noRegistrarConfigPath)
	defer f.close()

	// Register with nil request
	_, err := f.caClient.Register(nil)
	if err != api.ErrCARegistrarNotFound {
		t.Fatalf("Expected ErrCARegistrarNotFound, got: %v", err)
	}

	// Register without registration name parameter
	_, err = f.caClient.Register(&api.RegistrationRequest{})
	if err != api.ErrCARegistrarNotFound {
		t.Fatalf("Expected ErrCARegistrarNotFound, got: %v", err)
	}

	// Register with valid request
	var attributes []api.Attribute
	attributes = append(attributes, api.Attribute{Key: "test1", Value: "test2"})
	attributes = append(attributes, api.Attribute{Key: "test2", Value: "test3"})
	_, err = f.caClient.Register(&api.RegistrationRequest{Name: "test", Affiliation: "test", Attributes: attributes})
	if err != api.ErrCARegistrarNotFound {
		t.Fatalf("Expected ErrCARegistrarNotFound, got: %v", err)
	}
}

// TestRevoke will test multiple revoking a user with a nil request or a nil user
// TODO - improve Revoke test coverage
func TestRevoke(t *testing.T) {

	f := textFixture{}
	f.setup("")
	defer f.close()

	// Revoke with nil request
	_, err := f.caClient.Revoke(nil)
	if err == nil {
		t.Fatalf("Expected error with nil request")
	}

	mockKey := bccspwrapper.GetKey(&mocks.MockKey{})
	user := mocks.NewMockSigningIdentity("test", "test")
	user.SetEnrollmentCertificate(readCert(t))
	user.SetPrivateKey(mockKey)

	_, err = f.caClient.Revoke(&api.RevocationRequest{})
	if err == nil {
		t.Fatalf("Expected decoding error with test cert")
	}
}

// TestCAConfigError will test CAClient creation with bad CAConfig
func TestCAConfigError(t *testing.T) {

	f := textFixture{}
	f.setup("")
	defer f.close()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockConfig := mockCore.NewMockConfig(mockCtrl)

	mockConfig.EXPECT().NetworkConfig().Return(f.config.NetworkConfig()).AnyTimes()
	mockConfig.EXPECT().CryptoConfigPath().Return(f.config.CryptoConfigPath()).AnyTimes()
	mockConfig.EXPECT().CAConfig(org1).Return(nil, errors.New("CAConfig error"))
	mockConfig.EXPECT().CredentialStorePath().Return(dummyUserStorePath).AnyTimes()

	userStore := &mocks.MockUserStore{}
	_, err := NewCAClient(org1, f.identityManager, userStore, f.cryptoSuite, mockConfig)
	if err == nil || !strings.Contains(err.Error(), "CAConfig error") {
		t.Fatalf("Expected error from CAConfig. Got: %v", err)
	}
}

// TestCAServerCertPathsError will test CAClient creation with missing CAServerCertPaths
func TestCAServerCertPathsError(t *testing.T) {

	f := textFixture{}
	f.setup("")
	defer f.close()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockConfig := mockCore.NewMockConfig(mockCtrl)
	mockConfig.EXPECT().NetworkConfig().Return(f.config.NetworkConfig()).AnyTimes()
	mockConfig.EXPECT().CryptoConfigPath().Return(f.config.CryptoConfigPath()).AnyTimes()
	mockConfig.EXPECT().CAConfig(org1).Return(&core.CAConfig{}, nil).AnyTimes()
	mockConfig.EXPECT().CredentialStorePath().Return(dummyUserStorePath).AnyTimes()
	mockConfig.EXPECT().CAServerCertPaths(org1).Return(nil, errors.New("CAServerCertPaths error"))

	userStore := &mocks.MockUserStore{}
	_, err := NewCAClient(org1, f.identityManager, userStore, f.cryptoSuite, mockConfig)
	if err == nil || !strings.Contains(err.Error(), "CAServerCertPaths error") {
		t.Fatalf("Expected error from CAServerCertPaths. Got: %v", err)
	}
}

// TestCAClientCertPathError will test CAClient creation with missing CAClientCertPath
func TestCAClientCertPathError(t *testing.T) {

	f := textFixture{}
	f.setup("")
	defer f.close()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockConfig := mockCore.NewMockConfig(mockCtrl)
	mockConfig.EXPECT().NetworkConfig().Return(f.config.NetworkConfig()).AnyTimes()
	mockConfig.EXPECT().CryptoConfigPath().Return(f.config.CryptoConfigPath()).AnyTimes()
	mockConfig.EXPECT().CAConfig(org1).Return(&core.CAConfig{}, nil).AnyTimes()
	mockConfig.EXPECT().CredentialStorePath().Return(dummyUserStorePath).AnyTimes()
	mockConfig.EXPECT().CAServerCertPaths(org1).Return([]string{"test"}, nil)
	mockConfig.EXPECT().CAClientCertPath(org1).Return("", errors.New("CAClientCertPath error"))

	userStore := &mocks.MockUserStore{}
	_, err := NewCAClient(org1, f.identityManager, userStore, f.cryptoSuite, mockConfig)
	if err == nil || !strings.Contains(err.Error(), "CAClientCertPath error") {
		t.Fatalf("Expected error from CAClientCertPath. Got: %v", err)
	}
}

// TestCAClientKeyPathError will test CAClient creation with missing CAClientKeyPath
func TestCAClientKeyPathError(t *testing.T) {

	f := textFixture{}
	f.setup("")
	defer f.close()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockConfig := mockCore.NewMockConfig(mockCtrl)
	mockConfig.EXPECT().NetworkConfig().Return(f.config.NetworkConfig()).AnyTimes()
	mockConfig.EXPECT().CryptoConfigPath().Return(f.config.CryptoConfigPath()).AnyTimes()
	mockConfig.EXPECT().CAConfig(org1).Return(&core.CAConfig{}, nil).AnyTimes()
	mockConfig.EXPECT().CredentialStorePath().Return(dummyUserStorePath).AnyTimes()
	mockConfig.EXPECT().CAServerCertPaths(org1).Return([]string{"test"}, nil)
	mockConfig.EXPECT().CAClientCertPath(org1).Return("", nil)
	mockConfig.EXPECT().CAClientKeyPath(org1).Return("", errors.New("CAClientKeyPath error"))

	userStore := &mocks.MockUserStore{}
	_, err := NewCAClient(org1, f.identityManager, userStore, f.cryptoSuite, mockConfig)
	if err == nil || !strings.Contains(err.Error(), "CAClientKeyPath error") {
		t.Fatalf("Expected error from CAClientKeyPath. Got: %v", err)
	}
}

// TestInterfaces will test if the interface instantiation happens properly, ie no nil returned
func TestInterfaces(t *testing.T) {
	var apiClient api.CAClient
	var cl CAClientImpl

	apiClient = &cl
	if apiClient == nil {
		t.Fatalf("this shouldn't happen.")
	}
}
