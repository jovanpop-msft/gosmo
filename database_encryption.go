package gosmo

import (
	"context"
	"database/sql"
	"fmt"
)

// ============================================================
// Transparent data encryption (sys.dm_database_encryption_keys — SSMS's
// Manage Database Encryption dialog)
// ============================================================

// DatabaseEncryptionKey is a database's TDE key, from
// sys.dm_database_encryption_keys. The key is protected by a certificate or
// an asymmetric key in master; which one is found by the encryptor's
// thumbprint, so a protector the login cannot see in master leaves both
// names empty.
type DatabaseEncryptionKey struct {
	// KeyAlgorithm and KeyLength are what the server reports ("AES", 256).
	KeyAlgorithm string
	KeyLength    int
	// EncryptionState is the DMV's encryption_state: 1 unencrypted,
	// 2 encryption in progress, 3 encrypted, 4 key change, 5 decryption in
	// progress, 6 protection change.
	EncryptionState int
	// ServerCertificate or ServerAsymmetricKey names the protector in master.
	ServerCertificate   string
	ServerAsymmetricKey string
}

// Algorithm is the key's CREATE DATABASE ENCRYPTION KEY keyword — AES_128,
// AES_192, AES_256 or TRIPLE_DES_3KEY — or "" for anything else.
func (k *DatabaseEncryptionKey) Algorithm() string {
	switch {
	case k.KeyAlgorithm == "AES" && (k.KeyLength == 128 || k.KeyLength == 192 || k.KeyLength == 256):
		return fmt.Sprintf("AES_%d", k.KeyLength)
	case k.KeyAlgorithm == "TRIPLE_DES_3KEY", k.KeyAlgorithm == "TRIPLE_DES" && k.KeyLength == 192:
		return "TRIPLE_DES_3KEY"
	}
	return ""
}

// EncryptionKey returns the database's TDE key, or ErrNotFound when it has
// none. The DMV needs VIEW SERVER STATE (VIEW SERVER SECURITY STATE from
// SQL Server 2022); without it the read fails with Msg 300, which
// IsPermissionDenied recognises.
//
// Read at server scope: the DMV lists every database's key, and the
// protector lives in master, not in the database.
func (d *Database) EncryptionKey(ctx context.Context) (*DatabaseEncryptionKey, error) {
	k := &DatabaseEncryptionKey{}
	var cert, asym sql.NullString
	err := d.server.queryRow(ctx, func(row *sql.Row) error {
		return row.Scan(&k.KeyAlgorithm, &k.KeyLength, &k.EncryptionState, &cert, &asym)
	}, `
SELECT k.key_algorithm, k.key_length, k.encryption_state, c.name, a.name
FROM   sys.dm_database_encryption_keys k
LEFT   JOIN master.sys.certificates c ON c.thumbprint = k.encryptor_thumbprint
LEFT   JOIN master.sys.asymmetric_keys a ON a.thumbprint = k.encryptor_thumbprint
WHERE  k.database_id = DB_ID(@p1)`, d.Name)
	if err != nil {
		return nil, rowErr(err, notFoundf("gosmo: database %q has no encryption key", d.Name),
			fmt.Sprintf("encryption key of %q", d.Name))
	}
	k.ServerCertificate, k.ServerAsymmetricKey = cert.String, asym.String
	return k, nil
}
