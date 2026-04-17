package services

import (
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/pem"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/ssh"
)

// TestCAServiceNewCAServiceLoadsKeyFromDB tests that NewCAService successfully
// loads the CA key from the database and validates the key format.
func TestCAServiceNewCAServiceLoadsKeyFromDB(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupTestDB(t)
	defer db.Close()

	// Insert a valid CA key into the database
	privPEM, pubPEM := generateTestKeyPair(t)
	insertTestCAKey(t, db, privPEM, pubPEM)

	service, err := NewCAService(db)
	if err != nil {
		t.Fatalf("NewCAService failed: %v", err)
	}

	if service == nil {
		t.Fatal("NewCAService returned nil")
	}

	// Verify keys are loaded
	pubKey := service.GetPublicKey()
	if len(pubKey) == 0 {
		t.Fatal("public key not loaded")
	}
}

// TestCAServiceNewCAServiceNoKeyInDB tests that NewCAService returns an error
// when no active key is found in the database.
func TestCAServiceNewCAServiceNoKeyInDB(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupTestDB(t)
	defer db.Close()

	service, err := NewCAService(db)
	if err == nil {
		t.Fatal("expected error when no CA key exists, got nil")
	}

	if service != nil {
		t.Fatal("expected nil service when error occurs")
	}

	if err.Error() != "no active CA key found in database" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestCAServiceGenerateCertificate tests that GenerateCertificate creates
// a valid SSH certificate with correct TTL and principals.
func TestCAServiceGenerateCertificate(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupTestDB(t)
	defer db.Close()

	// Insert a valid CA key
	privPEM, pubPEM := generateTestKeyPair(t)
	insertTestCAKey(t, db, privPEM, pubPEM)

	service, err := NewCAService(db)
	if err != nil {
		t.Fatalf("NewCAService failed: %v", err)
	}

	userID := "user123"
	deviceID := "device456"
	durationMinutes := 60

	certBytes, err := service.GenerateCertificate(userID, deviceID, durationMinutes)
	if err != nil {
		t.Fatalf("GenerateCertificate failed: %v", err)
	}

	if len(certBytes) == 0 {
		t.Fatal("generated certificate is empty")
	}

	// Parse the generated certificate to validate structure
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(certBytes)
	if err != nil {
		t.Fatalf("failed to parse generated certificate: %v", err)
	}

	cert, ok := pubKey.(*ssh.Certificate)
	if !ok {
		t.Fatal("generated key is not a certificate")
	}

	// Verify certificate properties
	if cert.CertType != ssh.UserCert {
		t.Errorf("certificate type: expected %d, got %d", ssh.UserCert, cert.CertType)
	}

	if cert.KeyId != userID {
		t.Errorf("KeyId: expected %q, got %q", userID, cert.KeyId)
	}

	// Check principals
	if len(cert.ValidPrincipals) != 2 {
		t.Fatalf("expected 2 principals, got %d", len(cert.ValidPrincipals))
	}

	if cert.ValidPrincipals[0] != userID {
		t.Errorf("principal[0]: expected %q, got %q", userID, cert.ValidPrincipals[0])
	}

	if cert.ValidPrincipals[1] != deviceID {
		t.Errorf("principal[1]: expected %q, got %q", deviceID, cert.ValidPrincipals[1])
	}

	// Verify TTL (allow 1 second margin for test execution time)
	expectedValidBefore := uint64(time.Now().Add(time.Duration(durationMinutes) * time.Minute).Unix())
	if cert.ValidBefore < expectedValidBefore-1 || cert.ValidBefore > expectedValidBefore+5 {
		t.Errorf("ValidBefore mismatch: expected ~%d, got %d", expectedValidBefore, cert.ValidBefore)
	}

	if cert.ValidAfter > uint64(time.Now().Unix())+2 {
		t.Errorf("ValidAfter should be ~now, got %d (now=%d)", cert.ValidAfter, time.Now().Unix())
	}
}

// TestCAServiceGetPublicKey tests that GetPublicKey returns the correct public key.
func TestCAServiceGetPublicKey(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupTestDB(t)
	defer db.Close()

	privPEM, pubPEM := generateTestKeyPair(t)
	insertTestCAKey(t, db, privPEM, pubPEM)

	service, err := NewCAService(db)
	if err != nil {
		t.Fatalf("NewCAService failed: %v", err)
	}

	retrievedPubKey := service.GetPublicKey()

	if len(retrievedPubKey) == 0 {
		t.Fatal("GetPublicKey returned empty key")
	}

	// Verify it's a valid SSH public key
	parsedKey, _, _, _, err := ssh.ParseAuthorizedKey(retrievedPubKey)
	if err != nil {
		t.Fatalf("failed to parse retrieved public key: %v", err)
	}

	if parsedKey.Type() != ssh.KeyAlgoED25519 {
		t.Errorf("key type: expected %s, got %s", ssh.KeyAlgoED25519, parsedKey.Type())
	}
}

// TestCAServiceGenerateCertificateNoKeyLoaded tests that GenerateCertificate
// fails gracefully when the private key is not loaded.
func TestCAServiceGenerateCertificateNoKeyLoaded(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a service with a mock DB that has no keys
	service := &CAService{db: db}

	cert, err := service.GenerateCertificate("user", "device", 60)
	if err == nil {
		t.Fatal("expected error when key not loaded")
	}

	if len(cert) > 0 {
		t.Fatal("expected empty certificate on error")
	}

	if err.Error() != "CA private key not loaded" {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestCAServiceCertificateExpiration tests that generated certificates
// have the correct expiration time.
func TestCAServiceCertificateExpiration(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupTestDB(t)
	defer db.Close()

	privPEM, pubPEM := generateTestKeyPair(t)
	insertTestCAKey(t, db, privPEM, pubPEM)

	service, err := NewCAService(db)
	if err != nil {
		t.Fatalf("NewCAService failed: %v", err)
	}

	// Generate with a short TTL for testing
	durationMinutes := 5
	startTime := time.Now()

	certBytes, err := service.GenerateCertificate("user", "device", durationMinutes)
	if err != nil {
		t.Fatalf("GenerateCertificate failed: %v", err)
	}

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(certBytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	cert := pubKey.(*ssh.Certificate)

	// Check that ValidAfter is close to now
	if cert.ValidAfter < uint64(startTime.Unix()) || cert.ValidAfter > uint64(startTime.Unix())+2 {
		t.Errorf("ValidAfter not within 2 seconds of now")
	}

	// Check that ValidBefore is approximately 5 minutes from now
	expectedExpiry := startTime.Add(5 * time.Minute)
	if cert.ValidBefore < uint64(expectedExpiry.Unix())-2 || cert.ValidBefore > uint64(expectedExpiry.Unix())+2 {
		t.Errorf("ValidBefore not within 2 seconds of expected (now+5min)")
	}
}

// TestCAServiceVerifyCertificate tests that VerifyCertificate validates
// certificate structure and principals.
func TestCAServiceVerifyCertificate(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupTestDB(t)
	defer db.Close()

	privPEM, pubPEM := generateTestKeyPair(t)
	insertTestCAKey(t, db, privPEM, pubPEM)

	service, err := NewCAService(db)
	if err != nil {
		t.Fatalf("NewCAService failed: %v", err)
	}

	// Generate a certificate
	certBytes, err := service.GenerateCertificate("testuser", "testdevice", 60)
	if err != nil {
		t.Fatalf("GenerateCertificate failed: %v", err)
	}

	// Verify the certificate
	principals, err := service.VerifyCertificate(certBytes)
	if err != nil {
		t.Fatalf("VerifyCertificate failed: %v", err)
	}

	if len(principals) != 2 {
		t.Fatalf("expected 2 principals, got %d", len(principals))
	}

	if principals[0] != "testuser" || principals[1] != "testdevice" {
		t.Errorf("unexpected principals: %v", principals)
	}
}

// TestCAServiceGetPublicKeySafeCopy tests that GetPublicKey returns a safe
// copy that doesn't allow external modification of internal state.
func TestCAServiceGetPublicKeySafeCopy(t *testing.T) {
	if !hasTestDB() {
		t.Skip("skipping: test database not available")
	}

	db := setupTestDB(t)
	defer db.Close()

	privPEM, pubPEM := generateTestKeyPair(t)
	insertTestCAKey(t, db, privPEM, pubPEM)

	service, err := NewCAService(db)
	if err != nil {
		t.Fatalf("NewCAService failed: %v", err)
	}

	// Get public key
	key1 := service.GetPublicKey()
	key2 := service.GetPublicKey()

	// Modify the returned slice
	if len(key1) > 0 {
		key1[0] = 0xFF
	}

	// Second retrieval should not be affected
	key3 := service.GetPublicKey()
	if len(key2) > 0 && len(key3) > 0 && key2[0] == key3[0] {
		// Keys should still be the same
		if key2[0] == 0xFF {
			t.Fatal("external modification affected internal key storage")
		}
	}
}

// TestCAServiceGenerateCertificateStructure is a unit test that doesn't require DB
// and tests that we can properly generate and validate certificate structure.
func TestCAServiceGenerateCertificateStructure(t *testing.T) {
	// This test generates a key pair and validates certificate creation
	privPEM, pubPEM := generateTestKeyPair(t)

	// Create a service without loading from DB
	service := &CAService{
		privateKey: []byte(privPEM),
		publicKey:  []byte(pubPEM),
	}

	certBytes, err := service.GenerateCertificate("user", "device", 60)
	if err != nil {
		t.Fatalf("GenerateCertificate failed: %v", err)
	}

	if len(certBytes) == 0 {
		t.Fatal("certificate is empty")
	}

	// Verify the cert structure
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(certBytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	cert, ok := pubKey.(*ssh.Certificate)
	if !ok {
		t.Fatal("not a certificate")
	}

	if cert.CertType != ssh.UserCert {
		t.Errorf("cert type mismatch: expected UserCert")
	}

	if len(cert.ValidPrincipals) != 2 {
		t.Errorf("expected 2 principals, got %d", len(cert.ValidPrincipals))
	}
}

// Helper functions

// setupTestDB creates a PostgreSQL database connection for testing.
// It uses the JENNGATE_* environment variables to connect.
// For CI/CD, provide a test PostgreSQL instance.
func setupTestDB(t *testing.T) *sql.DB {
	// Use environment variables for test database connection
	// Default to localhost for local testing
	connStr := "postgresql://jenngate:jenngate@localhost:5432/jenngate_test?sslmode=disable"

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Skipf("skipping test: could not open test database: %v", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		t.Skipf("skipping test: could not connect to test database: %v", err)
	}

	// Create the gate_ca_keys table if it doesn't exist
	schema := `
	CREATE TABLE IF NOT EXISTS gate_ca_keys (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		key_type TEXT NOT NULL,
		public_key_pem TEXT NOT NULL,
		private_key_pem_encrypted TEXT NOT NULL,
		key_serial TEXT NOT NULL,
		rotated_at TIMESTAMP,
		expires_at TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := db.Exec(schema); err != nil {
		t.Skipf("skipping test: could not create schema: %v", err)
	}

	// Clean up any existing test data
	db.Exec("DELETE FROM gate_ca_keys;")

	return db
}

// generateTestKeyPair generates a test Ed25519 key pair and returns OpenSSH-encoded strings.
func generateTestKeyPair(t *testing.T) (privPEM, pubPEM string) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test key pair: %v", err)
	}

	// Encode private key as OpenSSH format
	privPEMBlock, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	privPEM = string(pem.EncodeToMemory(privPEMBlock))

	pubKey, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("failed to create public key: %v", err)
	}
	pubPEM = string(ssh.MarshalAuthorizedKey(pubKey))

	return privPEM, pubPEM
}

// insertTestCAKey inserts a test CA key into the test database.
func insertTestCAKey(t *testing.T, db *sql.DB, privPEM, pubPEM string) {
	query := `
	INSERT INTO gate_ca_keys (key_type, public_key_pem, private_key_pem_encrypted, key_serial)
	VALUES ($1, $2, $3, $4)
	`

	_, err := db.Exec(query, "ed25519", pubPEM, privPEM, "test-serial")
	if err != nil {
		t.Fatalf("failed to insert test CA key: %v", err)
	}
}

// hasTestDB checks if a test PostgreSQL database is available.
func hasTestDB() bool {
	connStr := "postgresql://jenngate:jenngate@localhost:5432/jenngate_test?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return false
	}
	defer db.Close()
	return db.Ping() == nil
}
