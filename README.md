# Steem Go SDK

> **⚠️ Under Construction**: This SDK is currently under active development. APIs may change in future versions.

A comprehensive Go SDK for interacting with the Steem blockchain, providing high-level abstractions for common operations while maintaining full compatibility with the Steem protocol.

## 🚀 Features

- **Complete API Coverage**: Access all Steem blockchain APIs (condenser_api, database_api, etc.)
- **Transaction Broadcasting**: Sign and broadcast transactions with automatic fee calculation
- **SignedCall Support**: Authenticated RPC calls compatible with steem-js
- **Key Management**: Secure private key handling and WIF import/export
- **Type Safety**: Strongly typed Go structs for all Steem data types
- **Easy Integration**: Simple, intuitive API designed for Go developers

## 📦 Installation

```bash
go get github.com/steemit/steemgosdk
```

## 🏃‍♂️ Quick Start

### Basic Setup

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/steemit/steemgosdk"
    "github.com/steemit/steemgosdk/consts"
)

func main() {
    // Create client
    client := steemgosdk.GetClient("https://api.steemit.com")
    client.AccountName = "your-account"
    
    // Import your private key (for signing operations)
    err := client.ImportWif(consts.POSTING_KEY, "your-posting-private-key")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println("✅ Client initialized successfully!")
}
```

### Making API Calls

```go
// Get account information
api := client.GetAPI()
accounts, err := api.GetAccounts([]string{"steemit"})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Account: %s, Reputation: %d\n", 
    accounts[0].Name, accounts[0].Reputation)
```

### Broadcasting Transactions

```go
// Vote on a post
broadcast := client.GetBroadcast()
err = broadcast.Vote("author", "permlink", 10000) // 100% upvote
if err != nil {
    log.Fatal(err)
}

fmt.Println("✅ Vote submitted successfully!")
```

### Authenticated Calls (SignedCall)

```go
// Make an authenticated API call
var result []map[string]interface{}
err = client.SignedCallWithResult(
    "condenser_api.get_accounts",
    []interface{}{[]string{"your-account"}},
    consts.ACTIVE_KEY,
    &result,
)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("✅ Authenticated call successful: %+v\n", result)
```

## 📚 Documentation

### Core Guides

- **[Examples](docs/examples.md)** - Comprehensive examples for all SDK features
- **[SignedCall API](docs/signed-call.md)** - Authenticated RPC calls documentation

### API Reference

The SDK is organized into several key components:

#### 🔌 Client (`steemgosdk.Client`)
- Main entry point for all SDK operations
- Handles connection management and authentication
- Provides access to API, Broadcast, and Auth instances

#### 🌐 API (`client.GetAPI()`)
- Access to all Steem blockchain APIs
- Read-only operations (get accounts, posts, etc.)
- Both regular and authenticated (SignedCall) methods

#### 📡 Broadcast (`client.GetBroadcast()`)
- Transaction signing and broadcasting
- Operations: vote, comment, transfer, etc.
- Automatic fee calculation and transaction preparation

#### 🔐 Auth (`client.GetAuth()`)
- Private key management
- WIF import/export
- Key derivation from username/password

## 🛠️ Advanced Usage

### Custom Node Configuration

```go
// Connect to a custom Steem node
client := steemgosdk.GetClient("https://your-custom-node.com")

// Configure timeout and other options
client.SetTimeout(30 * time.Second)
```

### Multiple Key Management

```go
auth := client.GetAuth()

// Import multiple keys for different operations
auth.ImportWif(consts.POSTING_KEY, "posting-private-key")
auth.ImportWif(consts.ACTIVE_KEY, "active-private-key")
auth.ImportWif(consts.MEMO_KEY, "memo-private-key")

// Generate keys from master password
keys, err := auth.GetPrivateKeys("username", "password", []string{"posting", "active"})
```

### Error Handling

```go
// The SDK provides detailed error information
err := broadcast.Vote("author", "permlink", 10000)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "missing required posting authority"):
        fmt.Println("❌ Missing posting key - please import your posting private key")
    case strings.Contains(err.Error(), "network"):
        fmt.Println("❌ Network error - check your connection")
    default:
        fmt.Printf("❌ Unexpected error: %v\n", err)
    }
}
```

## 🔗 Related Projects

- **[steemutil](https://github.com/steemit/steemutil)** - Low-level Steem utilities (used internally)
- **[steem-js](https://github.com/steemit/steem-js)** - JavaScript Steem library (SignedCall compatible)
- **[steem](https://github.com/steemit/steem)** - Official Steem blockchain implementation

## 🤝 Contributing

We welcome contributions! Please see our contributing guidelines for details.

### Development Setup

```bash
# Clone the repository
git clone https://github.com/steemit/steemgosdk.git
cd steemgosdk

# Install dependencies
go mod download

# Run tests
go test ./...

# Build examples
go build -o examples/vote_post examples/vote_post/main.go
```

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

- **Issues**: [GitHub Issues](https://github.com/steemit/steemgosdk/issues)
- **Documentation**: [API Documentation](https://pkg.go.dev/github.com/steemit/steemgosdk)
- **Examples**: See the [docs/examples.md](docs/examples.md) for comprehensive usage examples

---

**Built with ❤️ for the Steem community**