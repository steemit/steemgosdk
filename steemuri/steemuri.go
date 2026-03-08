package steemuri

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// EncodeProtocol defines the URI scheme for encoded steem URIs.
type EncodeProtocol string

const (
	ProtocolSteem    EncodeProtocol = "steem"
	ProtocolWebSteem EncodeProtocol = "web+steem"
	ProtocolExtSteem EncodeProtocol = "ext+steem"
)

var validProtocols = []string{"steem:", "web+steem:", "ext+steem:"}

// Parameters holds protocol parameters for a steem URI.
type Parameters struct {
	Signer      string
	Callback    string
	NoBroadcast bool
}

// DecodeResult holds the decoded transaction and parameters.
type DecodeResult struct {
	Tx     map[string]interface{}
	Params Parameters
}

// ResolveOptions provides values for resolving transaction placeholders.
type ResolveOptions struct {
	RefBlockNum     int
	RefBlockPrefix  int
	Expiration      string
	Signers         []string
	PreferredSigner string
}

// ResolveResult holds the resolved transaction and signer.
type ResolveResult struct {
	Tx     map[string]interface{}
	Signer string
}

// TransactionConfirmation holds confirmation data for callback resolution.
type TransactionConfirmation struct {
	Sig   string
	ID    string
	Block int
	Txn   int
}

// b64uEnc encodes a string to URL-safe base64 (+ → -, / → _, = → .).
func b64uEnc(data string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(data))
	r := strings.NewReplacer("+", "-", "/", "_", "=", ".")
	return r.Replace(encoded)
}

// b64uDec decodes a URL-safe base64 string back to plain text.
func b64uDec(data string) (string, error) {
	r := strings.NewReplacer("-", "+", "_", "/", ".", "=")
	standard := r.Replace(data)
	decoded, err := base64.StdEncoding.DecodeString(standard)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

// Decode parses a steem://, web+steem://, or ext+steem:// protocol link.
func Decode(steemUrl string) (*DecodeResult, error) {
	// Parse protocol manually since net/url doesn't handle custom schemes well for host extraction
	scheme, rest, found := strings.Cut(steemUrl, "://")
	if !found {
		return nil, fmt.Errorf("invalid protocol, expected one of %s", strings.Join(validProtocols, ", "))
	}
	protocol := scheme + ":" // e.g. "web+steem:"

	validProto := false
	for _, p := range validProtocols {
		if protocol == p {
			validProto = true
			break
		}
	}
	if !validProto {
		return nil, fmt.Errorf("invalid protocol, expected one of %s got '%s'", strings.Join(validProtocols, ", "), protocol)
	}

	// Split rest into path and query
	pathPart, queryStr, _ := strings.Cut(rest, "?")

	// Parse host and path segments: "sign/type/payload"
	segments := strings.SplitN(pathPart, "/", 3)
	if len(segments) < 3 || segments[0] != "sign" {
		return nil, fmt.Errorf("invalid action, expected 'sign' got '%s'", segments[0])
	}

	action := segments[1]
	rawPayload := segments[2]

	payloadStr, err := b64uDec(rawPayload)
	if err != nil {
		return nil, fmt.Errorf("invalid payload: %w", err)
	}

	var payload interface{}
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		return nil, fmt.Errorf("invalid payload: %w", err)
	}

	var tx map[string]interface{}
	switch action {
	case "tx":
		var ok bool
		tx, ok = payload.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid payload: expected object for tx action")
		}
	case "op":
		tx = map[string]interface{}{
			"ref_block_num":    "__ref_block_num",
			"ref_block_prefix": "__ref_block_prefix",
			"expiration":       "__expiration",
			"extensions":       []interface{}{},
			"operations":       []interface{}{payload},
		}
	case "ops":
		opsSlice, ok := payload.([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid payload: expected array for ops action")
		}
		tx = map[string]interface{}{
			"ref_block_num":    "__ref_block_num",
			"ref_block_prefix": "__ref_block_prefix",
			"expiration":       "__expiration",
			"extensions":       []interface{}{},
			"operations":       opsSlice,
		}
	default:
		return nil, fmt.Errorf("invalid signing action '%s'", action)
	}

	params := Parameters{}
	qp, _ := url.ParseQuery(queryStr)
	if cb := qp.Get("cb"); cb != "" {
		cbDecoded, err := b64uDec(cb)
		if err != nil {
			return nil, fmt.Errorf("invalid callback parameter: %w", err)
		}
		params.Callback = cbDecoded
	}
	if qp.Has("nb") {
		params.NoBroadcast = true
	}
	if s := qp.Get("s"); s != "" {
		params.Signer = s
	}

	return &DecodeResult{Tx: tx, Params: params}, nil
}

var resolvePattern = regexp.MustCompile(`__(ref_block_(num|prefix)|expiration|signer)`)

// walk recursively replaces placeholder strings in a value tree.
func walk(val interface{}, ctx map[string]interface{}) interface{} {
	switch v := val.(type) {
	case string:
		return resolvePattern.ReplaceAllStringFunc(v, func(m string) string {
			if replacement, ok := ctx[m]; ok {
				return fmt.Sprintf("%v", replacement)
			}
			return m
		})
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = walk(item, ctx)
		}
		return result
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, item := range v {
			result[k] = walk(item, ctx)
		}
		return result
	default:
		return val
	}
}

// ResolveTransaction resolves placeholders in a transaction.
func ResolveTransaction(utx map[string]interface{}, params Parameters, options ResolveOptions) (*ResolveResult, error) {
	signer := params.Signer
	if signer == "" {
		signer = options.PreferredSigner
	}

	found := false
	for _, s := range options.Signers {
		if s == signer {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("signer '%s' not available", signer)
	}

	ctx := map[string]interface{}{
		"__ref_block_num":    options.RefBlockNum,
		"__ref_block_prefix": options.RefBlockPrefix,
		"__expiration":       options.Expiration,
		"__signer":           signer,
	}

	resolved, ok := walk(utx, ctx).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("internal error: walk returned unexpected type")
	}
	return &ResolveResult{Tx: resolved, Signer: signer}, nil
}

var callbackResolvePattern = regexp.MustCompile(`\{\{(sig|id|block|txn)\}\}`)

// ResolveCallback resolves template variables in a callback URL.
func ResolveCallback(callbackURL string, ctx TransactionConfirmation) string {
	lookup := map[string]string{
		"sig":   ctx.Sig,
		"id":    ctx.ID,
		"block": fmt.Sprintf("%d", ctx.Block),
		"txn":   fmt.Sprintf("%d", ctx.Txn),
	}
	return callbackResolvePattern.ReplaceAllStringFunc(callbackURL, func(m string) string {
		key := m[2 : len(m)-2]
		if v, ok := lookup[key]; ok {
			return v
		}
		return ""
	})
}

// encodeParameters encodes Parameters to a query string.
func encodeParameters(params Parameters) string {
	var parts []string
	if params.NoBroadcast {
		parts = append(parts, "nb=")
	}
	if params.Signer != "" {
		parts = append(parts, "s="+params.Signer)
	}
	if params.Callback != "" {
		parts = append(parts, "cb="+b64uEnc(params.Callback))
	}
	if len(parts) == 0 {
		return ""
	}
	return "?" + strings.Join(parts, "&")
}

// encodeJSON serializes data to JSON then base64u encodes it.
func encodeJSON(data interface{}) (string, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal data: %w", err)
	}
	return b64uEnc(string(b)), nil
}

// EncodeTx encodes a Steem transaction to a steem URI.
func EncodeTx(tx interface{}, params Parameters, protocol EncodeProtocol) (string, error) {
	if protocol == "" {
		protocol = ProtocolWebSteem
	}
	encoded, err := encodeJSON(tx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s://sign/tx/%s%s", protocol, encoded, encodeParameters(params)), nil
}

// EncodeOp encodes a single Steem operation to a steem URI.
func EncodeOp(op interface{}, params Parameters, protocol EncodeProtocol) (string, error) {
	if protocol == "" {
		protocol = ProtocolWebSteem
	}
	encoded, err := encodeJSON(op)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s://sign/op/%s%s", protocol, encoded, encodeParameters(params)), nil
}

// EncodeOps encodes multiple Steem operations to a steem URI.
func EncodeOps(ops interface{}, params Parameters, protocol EncodeProtocol) (string, error) {
	if protocol == "" {
		protocol = ProtocolWebSteem
	}
	encoded, err := encodeJSON(ops)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s://sign/ops/%s%s", protocol, encoded, encodeParameters(params)), nil
}
