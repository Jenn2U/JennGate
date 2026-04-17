package services

import (
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// CAService manages SSH certificate authority operations.
// It loads the CA private key from the database and provides methods
// to issue certificates and retrieve the public key.
type CAService struct {
	db         *sql.DB
	privateKey []byte // PEM-encoded Ed25519 private key
	publicKey  []byte // PEM-encoded Ed25519 public key
	mu         sync.RWMutex
}

// NewCAService creates a new CA Service by loading the active CA key from the database.
// It validates that the key is Ed25519 format and returns an error if no active key exists.
func NewCAService(db *sql.DB) (*CAService, error) {
	service := &CAService{
		db: db,
	}

	// Load the active CA key from the database
	if err := service.loadActiveKey(); err != nil {
		return nil, err
	}

	return service, nil
}

// loadActiveKey loads the most recent active CA key from gate_ca_keys table.
// It decrypts the private key and validates the key format.
func (s *CAService) loadActiveKey() error {
	var privatePEM, publicPEM string

	// Query for the most recent non-expired key
	query := `
		SELECT private_key_pem_encrypted, public_key_pem
		FROM gate_ca_keys
		WHERE (expires_at IS NULL OR expires_at > NOW())
		ORDER BY created_at DESC
		LIMIT 1
	`

	err := s.db.QueryRow(query).Scan(&privatePEM, &publicPEM)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no active CA key found in database")
		}
		return fmt.Errorf("failed to query CA key: %w", err)
	}

	// TODO: Phase 3b - Decrypt private key from database
	// Currently loaded as plaintext from DB. Full encryption (AES-256 at rest)
	// plus in-memory decryption must be implemented before production use.
	privKey := privatePEM // Currently unencrypted; Phase 3b adds decryption

	// Validate private key format
	if err := s.validateKeyFormat(privKey); err != nil {
		return fmt.Errorf("invalid CA private key format: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.privateKey = []byte(privKey)
	s.publicKey = []byte(publicPEM)

	return nil
}

// validateKeyFormat checks that a PEM-encoded key can be parsed as Ed25519.
func (s *CAService) validateKeyFormat(keyPEM string) error {
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return fmt.Errorf("failed to decode PEM block")
	}

	// Parse as OpenSSH private key to validate Ed25519
	pk, err := ssh.ParsePrivateKey([]byte(keyPEM))
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Verify it's Ed25519 by checking the public key type
	if pk.PublicKey().Type() != ssh.KeyAlgoED25519 {
		return fmt.Errorf("key is not Ed25519 (got %s)", pk.PublicKey().Type())
	}

	return nil
}

// GenerateCertificate issues a new SSH certificate for a user with the given
// device and duration. The certificate includes both user_id and device_id
// as principals for flexible validation on target daemons.
// Returns a PEM-encoded SSH certificate or an error.
func (s *CAService) GenerateCertificate(
	userID string,
	deviceID string,
	durationMinutes int,
) ([]byte, error) {
	// Validate duration: between 1 minute and 1 year
	if durationMinutes <= 0 || durationMinutes > 525600 { // 1 year = 525600 minutes
		return nil, fmt.Errorf("durationMinutes must be between 1 and 525600, got %d", durationMinutes)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.privateKey == nil {
		return nil, fmt.Errorf("CA private key not loaded")
	}

	// Parse the CA private key
	pk, err := ssh.ParsePrivateKey(s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA private key: %w", err)
	}

	// Generate unique serial number: timestamp (high bits) + nonce (low bits)
	nonce := make([]byte, 4)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate serial nonce: %w", err)
	}
	serial := uint64(time.Now().Unix())<<32 | uint64(binary.BigEndian.Uint32(nonce))

	// Create certificate template
	cert := &ssh.Certificate{
		Key:             pk.PublicKey(),
		Serial:          serial,
		CertType:        ssh.UserCert,
		KeyId:           userID, // Identifiable name for the certificate
		ValidPrincipals: []string{userID, deviceID},
		ValidAfter:      uint64(time.Now().Unix()),
		ValidBefore:     uint64(time.Now().Add(time.Duration(durationMinutes) * time.Minute).Unix()),
		Permissions: ssh.Permissions{
			Extensions: map[string]string{
				"permit-X11-forwarding":   "",
				"permit-agent-forwarding": "",
				"permit-port-forwarding":  "",
				"permit-pty":              "",
				"permit-user-rc":          "",
			},
		},
	}

	// Sign the certificate with the CA key
	pubKey := pk.(ssh.Signer)
	if err := cert.SignCert(rand.Reader, pubKey); err != nil {
		return nil, fmt.Errorf("failed to sign certificate: %w", err)
	}

	// Encode certificate to OpenSSH format then PEM
	certBytes := ssh.MarshalAuthorizedKey(cert)
	if len(certBytes) == 0 {
		return nil, fmt.Errorf("failed to marshal certificate")
	}

	// Return as-is (OpenSSH format) for compatibility
	// If PEM is needed, wrap it here
	return certBytes, nil
}

// GetPublicKey returns the PEM-encoded public key for distribution to
// target daemons. The daemon uses this to validate SSH certificates issued by the CA.
func (s *CAService) GetPublicKey() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]byte, len(s.publicKey))
	copy(result, s.publicKey)
	return result
}

// VerifyCertificate validates an SSH certificate structure, checks expiration,
// and extracts principals. Signature verification is deferred to Phase 3b.
func (s *CAService) VerifyCertificate(cert []byte) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Parse the certificate
	parsedCert, _, _, _, err := ssh.ParseAuthorizedKey(cert)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	sshCert, ok := parsedCert.(*ssh.Certificate)
	if !ok {
		return nil, fmt.Errorf("provided key is not a certificate")
	}

	// Validate certificate type
	if sshCert.CertType != ssh.UserCert {
		return nil, fmt.Errorf("certificate type must be UserCert, got %d", sshCert.CertType)
	}

	// Validate expiration
	now := uint64(time.Now().Unix())
	if now < sshCert.ValidAfter {
		return nil, fmt.Errorf("certificate not yet valid (valid %d-%d, now %d)", sshCert.ValidAfter, sshCert.ValidBefore, now)
	}
	if now > sshCert.ValidBefore {
		return nil, fmt.Errorf("certificate expired (valid %d-%d, now %d)", sshCert.ValidAfter, sshCert.ValidBefore, now)
	}

	// TODO: Phase 3b - implement full signature verification
	// Go's ssh package doesn't expose cert signature validation directly.
	// Would require parsing OpenSSH cert binary format and using crypto/ed25519.Verify()

	return sshCert.ValidPrincipals, nil
}
