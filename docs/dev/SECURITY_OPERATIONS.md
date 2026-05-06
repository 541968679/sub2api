# Security Operations

This document records operational procedures for credential rotation and account hardening. Do not store plaintext secrets, generated passwords, or bcrypt hashes in this file.

## Admin Password Rotation

`ADMIN_PASSWORD` is a first-run bootstrap value. After a deployment is installed, changing `ADMIN_PASSWORD` in `.env` or `config.yaml` does not update an existing admin account. Rotate an installed admin password by updating the target administrator row in PostgreSQL.

1. Generate a 32-character password and bcrypt hash on a trusted machine:

   ```bash
   python3 - <<'PY'
   import bcrypt, secrets, string

   alphabet = string.ascii_letters + string.digits + "!@#$%^&*()-_=+[]{}:,.?"
   while True:
       password = "".join(secrets.choice(alphabet) for _ in range(32))
       if all(any(c in group for c in password) for group in [
           string.ascii_lowercase,
           string.ascii_uppercase,
           string.digits,
           "!@#$%^&*()-_=+[]{}:,.?",
       ]):
           break

   print("PASSWORD=" + password)
   print("BCRYPT_HASH=" + bcrypt.hashpw(password.encode(), bcrypt.gensalt(rounds=12)).decode())
   PY
   ```

2. Update the intended admin account only. Replace `<ADMIN_EMAIL>` and `<BCRYPT_HASH>` before running:

   ```sql
   DO $$
   BEGIN
     IF EXISTS (
       SELECT 1 FROM information_schema.columns
       WHERE table_name = 'users' AND column_name = 'token_version'
     ) THEN
       UPDATE users
       SET password_hash = '<BCRYPT_HASH>',
           token_version = token_version + 1,
           updated_at = now()
       WHERE email = '<ADMIN_EMAIL>' AND role = 'admin';
     ELSE
       UPDATE users
       SET password_hash = '<BCRYPT_HASH>',
           updated_at = now()
       WHERE email = '<ADMIN_EMAIL>' AND role = 'admin';
     END IF;
   END $$;
   ```

3. Verify the affected row count is exactly one.

4. Store the plaintext password in a password manager. Do not commit the plaintext password or bcrypt hash.

5. If the deployment supports TOTP, enable TOTP for administrator accounts after password rotation.

## Incident Checklist

- Rotate administrator passwords, administrator API keys, upstream account tokens, `JWT_SECRET`, `TOTP_ENCRYPTION_KEY`, database passwords, and Redis passwords when compromise is suspected.
- Restrict admin UI access by VPN, reverse-proxy allowlist, or equivalent network control.
- Review admin login activity, unusual source IPs, new API keys, account credential changes, billing changes, and high token usage windows.
- Treat any secret committed to Git history as compromised. Remove it from future commits and rotate the underlying credential.
