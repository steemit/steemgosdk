# Steem Go SDK Examples

This document provides practical examples for using the Steem Go SDK.

> **⚠️ Security Warning**: The private keys used in the examples below (`5JRaypasxMx1L97ZUX7YuC5Psb5EAbF821kkAGtBj7xCJFQcbLg`) are **test/example keys only**. They are publicly known and should **NEVER** be used in production. Always generate your own private keys from your account name and password using the `auth.ToWif()` or `auth.GetPrivateKeys()` methods.

## Table of Contents

1. [Basic Setup](#basic-setup)
2. [API Examples](#api-examples)
3. [Broadcast Examples](#broadcast-examples)
4. [Auth Examples](#auth-examples)
5. [Complete Examples](#complete-examples)

## Basic Setup

### Initialize Client

```go
package main

import (
    "fmt"
    "github.com/steemit/steemgosdk"
)

func main() {
    // Create a client pointing to a Steem node
    url := "https://api.steemit.com"
    client := steemgosdk.GetClient(url)
    
    // Set account name and WIFs if needed
    client.AccountName = "your-account-name"
    
    fmt.Println("Client initialized:", client.Url)
}
```

### Get API, Broadcast, and Auth Instances

```go
// Get API instance for making RPC calls
api := client.GetAPI()

// Get Broadcast instance for signing and broadcasting transactions
broadcast := client.GetBroadcast()

// Get Auth instance for authentication and key management
auth := client.GetAuth()
```

## API Examples

### Get Block Information

```go
package main

import (
    "fmt"
    "github.com/steemit/steemgosdk"
    "github.com/steemit/steemutil/protocol/api"
)

func main() {
    client := steemgosdk.GetClient("https://api.steemit.com")
    api := client.GetAPI()
    
    // Get block by number
    var block api.Block
    err := api.CallWithResult("condenser_api", "get_block", []interface{}{1}, &block)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    fmt.Printf("Block ID: %s\n", block.BlockID)
    fmt.Printf("Previous: %s\n", block.Previous)
    fmt.Printf("Timestamp: %s\n", block.Timestamp)
}
```

### Get Account Information

```go
package main

import (
    "encoding/json"
    "fmt"
    "github.com/steemit/steemgosdk"
)

func main() {
    client := steemgosdk.GetClient("https://api.steemit.com")
    api := client.GetAPI()
    
    // Get account information
    result, err := api.Call("condenser_api", "get_accounts", []interface{}{
        []interface{}{"steemit"},
    })
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    // Print raw result
    resultJSON, _ := json.MarshalIndent(result.Result, "", "  ")
    fmt.Println(string(resultJSON))
}
```

### Get Dynamic Global Properties

```go
package main

import (
    "fmt"
    "github.com/steemit/steemgosdk"
    "github.com/steemit/steemutil/protocol/api"
)

func main() {
    client := steemgosdk.GetClient("https://api.steemit.com")
    api := client.GetAPI()
    
    var dgp api.DynamicGlobalProperties
    err := api.CallWithResult("condenser_api", "get_dynamic_global_properties", 
        []interface{}{}, &dgp)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    fmt.Printf("Head Block Number: %d\n", dgp.HeadBlockNumber)
    fmt.Printf("Head Block ID: %s\n", dgp.HeadBlockId)
    fmt.Printf("Time: %s\n", dgp.Time)
}
```

### Get Content (Post/Comment)

```go
package main

import (
    "encoding/json"
    "fmt"
    "github.com/steemit/steemgosdk"
)

func main() {
    client := steemgosdk.GetClient("https://api.steemit.com")
    api := client.GetAPI()
    
    // Get a specific post/comment
    result, err := api.Call("condenser_api", "get_content", []interface{}{
        "steemit",
        "firstpost",
    })
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    resultJSON, _ := json.MarshalIndent(result.Result, "", "  ")
    fmt.Println(string(resultJSON))
}
```

## Broadcast Examples

### Vote on a Post

```go
package main

import (
    "fmt"
    "time"
    "github.com/steemit/steemgosdk"
    "github.com/steemit/steemutil/protocol"
)

func main() {
    client := steemgosdk.GetClient("https://api.steemit.com")
    broadcast := client.GetBroadcast()
    
    // Create a vote operation
    voteOp := &protocol.VoteOperation{
        Voter:    "your-account-name",
        Author:   "steemit",
        Permlink: "firstpost",
        Weight:   10000, // 100% vote weight (10000 = 100%)
    }
    
    // Your posting private key in WIF format
    // WARNING: This is a test/example key. NEVER use it in production!
    // Generate your own key using: auth.ToWif(accountName, password, "posting")
    postingWif := "5JRaypasxMx1L97ZUX7YuC5Psb5EAbF821kkAGtBj7xCJFQcbLg"
    
    // Broadcast the vote
    result, err := broadcast.SendWith(voteOp, postingWif)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    fmt.Printf("Transaction broadcasted: %s\n", string(result))
}
```

### Transfer STEEM or SBD

```go
package main

import (
    "fmt"
    "time"
    "github.com/steemit/steemgosdk"
    "github.com/steemit/steemutil/protocol"
)

func main() {
    client := steemgosdk.GetClient("https://api.steemit.com")
    broadcast := client.GetBroadcast()
    
    // Create a transfer operation
    transferOp := &protocol.TransferOperation{
        From:   "your-account-name",
        To:     "recipient-account",
        Amount: "1.000 STEEM",
        Memo:   "Hello from Steem Go SDK!",
    }
    
    // Your active private key in WIF format
    // WARNING: This is a test/example key. NEVER use it in production!
    activeWif := "5JRaypasxMx1L97ZUX7YuC5Psb5EAbF821kkAGtBj7xCJFQcbLg"
    
    // Broadcast the transfer
    result, err := broadcast.SendWith(transferOp, activeWif)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    fmt.Printf("Transfer broadcasted: %s\n", string(result))
}
```

### Transfer with Encrypted Memo

```go
package main

import (
    "fmt"
    "time"
    "github.com/steemit/steemgosdk"
    "github.com/steemit/steemutil/protocol"
)

func main() {
    client := steemgosdk.GetClient("https://api.steemit.com")
    broadcast := client.GetBroadcast()
    auth := client.GetAuth()
    
    // Recipient's memo public key
    recipientMemoKey := "STM8m5UgaFAAYQRuaNejYdS8FVLVp9Ss3K1qAVk5de6F8s3HnVbvA"
    
    // Your memo private key
    // WARNING: This is a test/example key. NEVER use it in production!
    yourMemoWif := "5JRaypasxMx1L97ZUX7YuC5Psb5EAbF821kkAGtBj7xCJFQcbLg"
    
    // Encrypt the memo (memos starting with '#' are encrypted)
    encryptedMemo, err := auth.EncodeMemo(yourMemoWif, recipientMemoKey, "#Secret message")
    if err != nil {
        fmt.Printf("Error encrypting memo: %v\n", err)
        return
    }
    
    // Create transfer with encrypted memo
    transferOp := &protocol.TransferOperation{
        From:   "your-account-name",
        To:     "recipient-account",
        Amount: "1.000 STEEM",
        Memo:   encryptedMemo,
    }
    
    // Your active private key
    // WARNING: This is a test/example key. NEVER use it in production!
    activeWif := "5JRaypasxMx1L97ZUX7YuC5Psb5EAbF821kkAGtBj7xCJFQcbLg"
    
    // Broadcast the transfer
    result, err := broadcast.SendWith(transferOp, activeWif)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    fmt.Printf("Transfer with encrypted memo broadcasted: %s\n", string(result))
}
```

### Create a Comment

```go
package main

import (
    "fmt"
    "time"
    "github.com/steemit/steemgosdk"
    "github.com/steemit/steemutil/protocol"
)

func main() {
    client := steemgosdk.GetClient("https://api.steemit.com")
    broadcast := client.GetBroadcast()
    
    // Create a comment operation
    commentOp := &protocol.CommentOperation{
        ParentAuthor:   "steemit",
        ParentPermlink: "firstpost",
        Author:         "your-account-name",
        Permlink:       "my-comment-permlink",
        Title:          "My Comment Title",
        Body:           "This is the body of my comment.",
        JsonMetadata:   `{"tags":["steem"],"app":"steemgosdk/1.0"}`,
    }
    
    // Your posting private key
    // WARNING: This is a test/example key. NEVER use it in production!
    postingWif := "5JRaypasxMx1L97ZUX7YuC5Psb5EAbF821kkAGtBj7xCJFQcbLg"
    
    // Broadcast the comment
    result, err := broadcast.SendWith(commentOp, postingWif)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    fmt.Printf("Comment broadcasted: %s\n", string(result))
}
```

### Multiple Operations in One Transaction

```go
package main

import (
    "fmt"
    "time"
    "github.com/steemit/steemgosdk"
    "github.com/steemit/steemutil/protocol"
)

func main() {
    client := steemgosdk.GetClient("https://api.steemit.com")
    broadcast := client.GetBroadcast()
    
    // Create multiple operations
    voteOp := &protocol.VoteOperation{
        Voter:    "your-account-name",
        Author:   "steemit",
        Permlink: "firstpost",
        Weight:   10000,
    }
    
    commentOp := &protocol.CommentOperation{
        ParentAuthor:   "steemit",
        ParentPermlink: "firstpost",
        Author:         "your-account-name",
        Permlink:       "my-comment-permlink",
        Title:          "My Comment",
        Body:           "Great post!",
        JsonMetadata:   "{}",
    }
    
    // Prepare private keys (posting key for both operations)
    // WARNING: This is a test/example key. NEVER use it in production!
    privKeys := map[string]string{
        "posting": "5JRaypasxMx1L97ZUX7YuC5Psb5EAbF821kkAGtBj7xCJFQcbLg",
    }
    
    // Broadcast multiple operations in one transaction
    result, err := broadcast.Send([]protocol.Operation{voteOp, commentOp}, privKeys)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    fmt.Printf("Transaction with multiple operations broadcasted: %s\n", string(result))
}
```

## Auth Examples

### Generate Keys from Account Name and Password

```go
package main

import (
    "fmt"
    "github.com/steemit/steemgosdk"
)

func main() {
    client := steemgosdk.GetClient("https://api.steemit.com")
    auth := client.GetAuth()
    
    accountName := "your-account-name"
    password := "your-password"
    roles := []string{"owner", "active", "posting", "memo"}
    
    // Generate public keys
    pubKeys, err := auth.GenerateKeys(accountName, password, roles)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    fmt.Printf("Owner key: %s\n", pubKeys["owner"])
    fmt.Printf("Active key: %s\n", pubKeys["active"])
    fmt.Printf("Posting key: %s\n", pubKeys["posting"])
    fmt.Printf("Memo key: %s\n", pubKeys["memo"])
}
```

### Get Private Keys (WIFs)

```go
package main

import (
    "fmt"
    "github.com/steemit/steemgosdk"
)

func main() {
    client := steemgosdk.GetClient("https://api.steemit.com")
    auth := client.GetAuth()
    
    accountName := "your-account-name"
    password := "your-password"
    roles := []string{"posting", "active"}
    
    // Get private keys and public keys
    keys, err := auth.GetPrivateKeys(accountName, password, roles)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    fmt.Printf("Posting WIF: %s\n", keys["posting"])
    fmt.Printf("Posting Public Key: %s\n", keys["postingPubkey"])
    fmt.Printf("Active WIF: %s\n", keys["active"])
    fmt.Printf("Active Public Key: %s\n", keys["activePubkey"])
}
```

### Convert Account Name and Password to WIF

```go
package main

import (
    "fmt"
    "github.com/steemit/steemgosdk"
)

func main() {
    client := steemgosdk.GetClient("https://api.steemit.com")
    auth := client.GetAuth()
    
    accountName := "your-account-name"
    password := "your-password"
    role := "posting"
    
    // Convert to WIF
    wif, err := auth.ToWif(accountName, password, role)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    fmt.Printf("WIF for %s role: %s\n", role, wif)
}
```

### Validate WIF and Public Key

```go
package main

import (
    "fmt"
    "github.com/steemit/steemgosdk"
)

func main() {
    client := steemgosdk.GetClient("https://api.steemit.com")
    auth := client.GetAuth()
    
    // WARNING: This is a test/example key. NEVER use it in production!
    wif := "5JRaypasxMx1L97ZUX7YuC5Psb5EAbF821kkAGtBj7xCJFQcbLg"
    pubKey := "STM8m5UgaFAAYQRuaNejYdS8FVLVp9Ss3K1qAVk5de6F8s3HnVbvA"
    
    // Check if WIF is valid
    isWif := auth.IsWif(wif)
    fmt.Printf("Is valid WIF: %v\n", isWif)
    
    // Check if public key is valid
    isPubkey := auth.IsPubkey(pubKey)
    fmt.Printf("Is valid public key: %v\n", isPubkey)
    
    // Verify WIF matches public key
    isValid := auth.WifIsValid(wif, pubKey)
    fmt.Printf("WIF matches public key: %v\n", isValid)
    
    // Convert WIF to public key
    derivedPubKey, err := auth.WifToPublic(wif)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    fmt.Printf("Derived public key: %s\n", derivedPubKey)
}
```

### Encrypt and Decrypt Memo

```go
package main

import (
    "fmt"
    "github.com/steemit/steemgosdk"
)

func main() {
    client := steemgosdk.GetClient("https://api.steemit.com")
    auth := client.GetAuth()
    
    // Sender's memo private key
    // WARNING: This is a test/example key. NEVER use it in production!
    senderMemoWif := "5JRaypasxMx1L97ZUX7YuC5Psb5EAbF821kkAGtBj7xCJFQcbLg"
    
    // Recipient's memo public key
    recipientMemoKey := "STM8m5UgaFAAYQRuaNejYdS8FVLVp9Ss3K1qAVk5de6F8s3HnVbvA"
    
    // Original message (must start with '#' to be encrypted)
    originalMemo := "#This is a secret message"
    
    // Encrypt memo
    encryptedMemo, err := auth.EncodeMemo(senderMemoWif, recipientMemoKey, originalMemo)
    if err != nil {
        fmt.Printf("Error encrypting: %v\n", err)
        return
    }
    fmt.Printf("Encrypted memo: %s\n", encryptedMemo)
    
    // Decrypt memo (recipient uses their memo private key)
    // WARNING: This is a test/example key. NEVER use it in production!
    recipientMemoWif := "5JRaypasxMx1L97ZUX7YuC5Psb5EAbF821kkAGtBj7xCJFQcbLg"
    decryptedMemo, err := auth.DecodeMemo(recipientMemoWif, encryptedMemo)
    if err != nil {
        fmt.Printf("Error decrypting: %v\n", err)
        return
    }
    fmt.Printf("Decrypted memo: %s\n", decryptedMemo)
}
```

## Complete Examples

### Complete Transfer Example

```go
package main

import (
    "fmt"
    "time"
    "github.com/steemit/steemgosdk"
    "github.com/steemit/steemutil/protocol"
)

func main() {
    // Initialize client
    url := "https://api.steemit.com"
    client := steemgosdk.GetClient(url)
    
    // Get instances
    api := client.GetAPI()
    broadcast := client.GetBroadcast()
    auth := client.GetAuth()
    
    // 1. Check account balance (optional)
    var account interface{}
    err := api.CallWithResult("condenser_api", "get_accounts", []interface{}{
        []interface{}{"your-account-name"},
    }, &account)
    if err != nil {
        fmt.Printf("Error getting account: %v\n", err)
        return
    }
    fmt.Println("Account retrieved successfully")
    
    // 2. Prepare transfer operation
    transferOp := &protocol.TransferOperation{
        From:   "your-account-name",
        To:     "recipient-account",
        Amount: "1.000 STEEM",
        Memo:   "Transfer from Steem Go SDK",
    }
    
    // 3. Get private key (in production, store securely)
    // WARNING: This is a test/example key. NEVER use it in production!
    activeWif := "5JRaypasxMx1L97ZUX7YuC5Psb5EAbF821kkAGtBj7xCJFQcbLg"
    
    // Or generate from account name and password
    // keys, _ := auth.GetPrivateKeys("your-account-name", "your-password", []string{"active"})
    // activeWif := keys["active"]
    
    // 4. Broadcast transaction
    result, err := broadcast.SendWith(transferOp, activeWif)
    if err != nil {
        fmt.Printf("Error broadcasting: %v\n", err)
        return
    }
    
    fmt.Printf("Transfer successful! Result: %s\n", string(result))
}
```

### Complete Vote and Comment Example

```go
package main

import (
    "fmt"
    "time"
    "github.com/steemit/steemgosdk"
    "github.com/steemit/steemutil/protocol"
)

func main() {
    // Initialize
    client := steemgosdk.GetClient("https://api.steemit.com")
    broadcast := client.GetBroadcast()
    auth := client.GetAuth()
    
    // Get posting key
    accountName := "your-account-name"
    password := "your-password"
    keys, err := auth.GetPrivateKeys(accountName, password, []string{"posting"})
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    postingWif := keys["posting"]
    
    // 1. Vote on a post
    voteOp := &protocol.VoteOperation{
        Voter:    accountName,
        Author:   "steemit",
        Permlink: "firstpost",
        Weight:   10000, // 100%
    }
    
    result, err := broadcast.SendWith(voteOp, postingWif)
    if err != nil {
        fmt.Printf("Error voting: %v\n", err)
        return
    }
    fmt.Printf("Vote broadcasted: %s\n", string(result))
    
    // 2. Comment on the same post
    commentOp := &protocol.CommentOperation{
        ParentAuthor:   "steemit",
        ParentPermlink: "firstpost",
        Author:         accountName,
        Permlink:       "my-comment-" + fmt.Sprintf("%d", time.Now().Unix()),
        Title:          "",
        Body:           "Great post! Thanks for sharing.",
        JsonMetadata:   `{"tags":["steem"],"app":"steemgosdk/1.0"}`,
    }
    
    result, err = broadcast.SendWith(commentOp, postingWif)
    if err != nil {
        fmt.Printf("Error commenting: %v\n", err)
        return
    }
    fmt.Printf("Comment broadcasted: %s\n", string(result))
}
```

## Notes

- **Security**: Never hardcode private keys or passwords in production code. Use environment variables or secure key management systems.
- **Test Private Keys**: The private key `5JRaypasxMx1L97ZUX7YuC5Psb5EAbF821kkAGtBj7xCJFQcbLg` used in examples is a **test/example key only**. It is publicly known and should **NEVER** be used in production. Always use your own private keys generated from your account name and password.
- **Error Handling**: Always check errors returned by SDK methods.
- **Network**: The examples use `https://api.steemit.com` as the default node. You can use any Steem node URL.
- **WIF Format**: Private keys should be in Wallet Import Format (WIF), starting with '5'.
- **Public Keys**: Public keys should be in Steem format, starting with 'STM'.
- **Memo Encryption**: Memos starting with '#' are automatically encrypted. Plain text memos are sent as-is.

## Additional Resources

- [Steem Documentation](https://developers.steem.io/)
- [Steem API Reference](https://developers.steem.io/apidefinitions/)
- [Steem Operations](https://developers.steem.io/apidefinitions/broadcast-ops)

