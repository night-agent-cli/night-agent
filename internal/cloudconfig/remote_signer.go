package cloudconfig

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/night-agent-cli/night-agent/internal/audit"
)

const remoteSignTimeout = 3 * time.Second

// RemoteSigner chiama POST /api/sign per ottenere la firma del cloud.
type RemoteSigner struct {
	cfg    *Config
	client *http.Client
}

// NewRemoteSigner crea un RemoteSigner dalla Config corrente.
func NewRemoteSigner(cfg *Config) *RemoteSigner {
	return &RemoteSigner{
		cfg:    cfg,
		client: &http.Client{Timeout: remoteSignTimeout},
	}
}

// SignFunc restituisce una audit.SignFunc che:
// - tenta firma remota (timeout 3s)
// - in caso di errore, fa fallback locale usando localSigner
func (r *RemoteSigner) SignFunc(localSigner *audit.Signer) audit.SignFunc {
	return func(e audit.Event) (string, string, error) {
		// calcola hash dell'evento (con sig="" e sig_source="" come fa Sign locale)
		e.Sig = ""
		e.SigSource = ""
		payload, err := json.Marshal(e)
		if err != nil {
			return r.fallback(localSigner, e, fmt.Errorf("marshal: %w", err))
		}
		hash := sha256.Sum256(payload)
		hashHex := hex.EncodeToString(hash[:])

		// chiama /api/sign
		sig, err := r.callSign(hashHex, e.ID)
		if err != nil {
			return r.fallback(localSigner, e, err)
		}
		return sig, "remote", nil
	}
}

type signRequest struct {
	MachineID string `json:"machine_id"`
	EventID   string `json:"event_id"`
	Hash      string `json:"hash"`
}

type signResponse struct {
	Sig string `json:"sig"`
}

func (r *RemoteSigner) callSign(hashHex, eventID string) (string, error) {
	body, err := json.Marshal(signRequest{
		MachineID: r.cfg.MachineID,
		EventID:   eventID,
		Hash:      hashHex,
	})
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), remoteSignTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.cfg.Endpoint+"/api/sign", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.cfg.Token)

	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("remote sign request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("remote sign: status %d", resp.StatusCode)
	}

	var res signResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	if res.Sig == "" {
		return "", fmt.Errorf("remote sign: empty sig")
	}
	return res.Sig, nil
}

func (r *RemoteSigner) fallback(localSigner *audit.Signer, e audit.Event, reason error) (string, string, error) {
	if localSigner == nil {
		return "", "local", fmt.Errorf("remote sign fallito e nessun signer locale: %w", reason)
	}
	signed, err := localSigner.Sign(e)
	if err != nil {
		return "", "local", fmt.Errorf("fallback locale: %w", err)
	}
	return signed.Sig, "local", nil
}
