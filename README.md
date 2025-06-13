xgolsw
========

[![Build Status](https://github.com/goplus/xgolsw/actions/workflows/go.yml/badge.svg)](https://github.com/goplus/xgolsw/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/goplus/xgolsw)](https://goreportcard.com/report/github.com/goplus/xgolsw)
[![GitHub release](https://img.shields.io/github/v/tag/goplus/xgolsw.svg?label=release)](https://github.com/goplus/xgolsw/releases)
[![Coverage Status](https://codecov.io/gh/goplus/xgolsw/branch/main/graph/badge.svg)](https://codecov.io/gh/goplus/xgolsw)
[![GoDoc](https://pkg.go.dev/badge/github.com/goplus/xgolsw.svg)](https://pkg.go.dev/github.com/goplus/xgolsw)

A lightweight XGo language server that runs in the browser using WebAssembly.

This project follows the [Language Server Protocol (LSP)](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/)
using [JSON-RPC 2.0](https://www.jsonrpc.org/specification) for message exchange. However, unlike traditional LSP
implementations that require a network transport layer, this project operates directly in the browser's memory space
through its API interfaces.

## Difference between `goxls` and `xgolsw`

* `goxls` runs locally, while `xgolsw` runs in the browser using WebAssembly.
* `goxls` supports a workspace (multiple projects), while `xgolsw` supports a single project.
* `goxls` supports mixed programming of Go and XGo, while `xgolsw` only supports a pure XGo project.

## Building from source

1. [Optional] Generate required package data:

  ```bash
  go generate ./internal/pkgdata
  ```

2. Build the project:

  ```bash
  GOOS=js GOARCH=wasm go build -trimpath -o spxls.wasm
  ```

## Usage

This project is a standard Go WebAssembly module. You can use it like any other Go WASM modules in your web applications.

For detailed API references, please check the [index.d.ts](index.d.ts) file.

## Supported LSP methods

| Category | Method | Purpose & Explanation |
|----------|--------|-----------------------|
| **Lifecycle Management** |||
|| [`initialize`](https://microsoft.github.io/language-server-protocol/specifications/base/0.9/specification/#initialize) | Performs initial handshake, establishes server capabilities and client configuration. |
|| [`initialized`](https://microsoft.github.io/language-server-protocol/specifications/base/0.9/specification/#initialized) | Marks completion of initialization process, enabling request processing. |
|| [`shutdown`](https://microsoft.github.io/language-server-protocol/specifications/base/0.9/specification/#shutdown) | *Protocol conformance only.* |
|| [`exit`](https://microsoft.github.io/language-server-protocol/specifications/base/0.9/specification/#exit) | *Protocol conformance only.* |
| **Document Synchronization** |||
|| [`textDocument/didOpen`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_didOpen) | Registers new document in server state and triggers initial diagnostics. |
|| [`textDocument/didChange`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_didChange) | Synchronizes document content changes between client and server. |
|| [`textDocument/didSave`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_didSave) | Processes document save events and triggers related operations. |
|| [`textDocument/didClose`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_didClose) | Removes document from server state and cleans up resources. |
| **Code Intelligence** |||
|| [`textDocument/hover`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_hover) | Shows types and documentation at cursor position. |
|| [`textDocument/completion`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_completion) | Generates context-aware code suggestions. |
|| [`textDocument/signatureHelp`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_signatureHelp) | Shows function/method signature information. |
| **Symbols & Navigation** |||
|| [`textDocument/declaration`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_declaration) | Finds symbol declarations. |
|| [`textDocument/definition`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_definition) | Locates symbol definitions across workspace. |
|| [`textDocument/typeDefinition`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_typeDefinition) | Navigates to type definitions of variables/fields. |
|| [`textDocument/implementation`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_implementation) | Locates implementations. |
|| [`textDocument/references`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_references) | Finds all references of a symbol. |
|| [`textDocument/documentHighlight`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_documentHighlight) | Highlights other occurrences of selected symbol. |
|| [`textDocument/documentLink`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_documentLink) | Provides clickable links within document content. |
| **Code Quality** |||
|| [`textDocument/publishDiagnostics`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_publishDiagnostics) | Reports code errors and warnings in real-time. |
|| [`textDocument/diagnostic`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_diagnostic) | Pulls diagnostics for documents on request (pull model). |
|| [`workspace/diagnostic`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#workspace_diagnostic) | Pulls diagnostics for all workspace documents on request. |
| **Code Modification** |||
|| [`textDocument/formatting`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_formatting) | Applies standardized formatting rules to document. |
|| [`textDocument/prepareRename`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_prepareRename) | Validates renaming possibility and returns valid range for the operation. |
|| [`textDocument/rename`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_rename) | Performs consistent symbol renaming across workspace. |
| **Semantic Features** |||
|| [`textDocument/semanticTokens/full`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#semanticTokens_fullRequest) | Provides semantic coloring for whole document. |
|| [`textDocument/inlayHint`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_inlayHint) | Provides inline hints such as parameter names and type annotations. |
| **Other** |||
|| [`workspace/executeCommand`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#workspace_executeCommand) | Executes [predefined commands](#predefined-commands) for workspace-specific operations. |

## Predefined commands

### Resource renaming

The `spx.renameResources` command enables renaming of resources referenced by string literals (e.g., `play "explosion"`)
across the workspace.

*Request:*

- method: [`workspace/executeCommand`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#workspace_executeCommand)
- params: [`ExecuteCommandParams`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#executeCommandParams)
defined as follows:

```typescript
interface ExecuteCommandParams {
  /**
   * The identifier of the actual command handler.
   */
  command: 'spx.renameResources'

  /**
   * Arguments that the command should be invoked with.
   */
  arguments: SpxRenameResourceParams[]
}
```

```typescript
/**
 * Parameters to rename an spx resource in the workspace.
 */
interface SpxRenameResourceParams {
  /**
   * The spx resource.
   */
  resource: SpxResourceIdentifier

  /**
   * The new name of the spx resource.
   */
  newName: string
}
```

```typescript
/**
 * The spx resource's identifier.
 */
interface SpxResourceIdentifier {
  /**
   * The spx resource's URI.
   */
  uri: SpxResourceUri
}
```

```typescript
/**
 * The spx resource's URI.
 *
 * @example
 * - `spx://resources/sounds/MySound`
 * - `spx://resources/sprites/MySprite`
 * - `spx://resources/sprites/MySprite/costumes/MyCostume`
 * - `spx://resources/sprites/MySprite/animations/MyAnimation`
 * - `spx://resources/backdrops/MyBackdrop`
 * - `spx://resources/widgets/MyWidget`
 */
type SpxResourceUri = string
```

*Response:*

- result: [`WorkspaceEdit`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#workspaceEdit)
  | `null` describing the modification to the workspace. `null` should be treated the same as
  [`WorkspaceEdit`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#workspaceEdit)
with no changes (no change was required).
- error: code and message set in case when rename could not be performed for any reason.

### Definition lookup

The `spx.getDefinitions` command retrieves definition identifiers at a given position in a document.

*Request:*

- method: [`workspace/executeCommand`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#workspace_executeCommand)
- params: [`ExecuteCommandParams`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#executeCommandParams)
defined as follows:

```typescript
interface ExecuteCommandParams {
  /**
   * The identifier of the actual command handler.
   */
  command: 'spx.getDefinitions'

  /**
   * Arguments that the command should be invoked with.
   */
  arguments: SpxGetDefinitionsParams[]
}
```

```typescript
/**
 * Parameters to get definitions at a specific position in a document.
 */
interface SpxGetDefinitionsParams extends TextDocumentPositionParams {}
```

*Response:*

- result: `SpxDefinitionIdentifier[]` | `null` describing the definitions found at the given position. `null` indicates
  no definitions were found.
- error: code and message set in case when definitions could not be retrieved for any reason.

```typescript
/**
 * The identifier of a definition.
 */
interface SpxDefinitionIdentifier {
  /**
   * Full name of source package.
   * If not provided, it's assumed to be kind-statement.
   * If `main`, it's the current user package.
   * Examples:
   * - `fmt`
   * - `github.com/goplus/spx/v2`
   * - `main`
   */
  package?: string;

  /**
   * Exported name of the definition.
   * If not provided, it's assumed to be kind-package.
   * Examples:
   * - `Println`
   * - `Sprite`
   * - `Sprite.turn`
   * - `for_statement_with_single_condition`
   */
  name?: string;

  /**
   * Overload Identifier.
   */
  overloadId?: string;
}
```

### Input slots lookup

The `spx.getInputSlots` command retrieves all modifiable items (input slots) in a document, which can be used to
provide UI controls for assisting users with code modifications.

*Request:*

- method: [`workspace/executeCommand`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#workspace_executeCommand)
- params: [`ExecuteCommandParams`](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#executeCommandParams)
defined as follows:

```typescript
interface ExecuteCommandParams {
  /**
   * The identifier of the actual command handler.
   */
  command: 'spx.getInputSlots'

  /**
   * Arguments that the command should be invoked with.
   */
  arguments: [SpxGetInputSlotsParams]
}
```

```typescript
/**
 * Parameters to get input slots in a document.
 */
interface SpxGetInputSlotsParams {
  /**
   * The text document identifier.
   */
  textDocument: TextDocumentIdentifier
}
```

*Response:*

- result: `SpxInputSlot[]` | `null` describing the input slots found in the document. `null` indicates no input slots
  were found.
- error: code and message set in case when input slots could not be retrieved for any reason.

```typescript
/**
 * Represents a modifiable item in the code.
 */
interface SpxInputSlot {
  /**
   * Kind of the slot.
   * - Value: Modifiable values that can be replaced with different values.
   * - Address: Modifiable operation objects that can be replaced with user-defined objects.
   */
  kind: SpxInputSlotKind

  /**
   * Info describing what inputs are accepted by the slot.
   */
  accept: SpxInputSlotAccept

  /**
   * Current input in the slot.
   */
  input: SpxInput

  /**
   * Names for available user-predefined identifiers.
   */
  predefinedNames: string[]

  /**
   * Range in code for the slot.
   */
  range: Range
}
```

```typescript
/**
 * The kind of input slot.
 */
enum SpxInputSlotKind {
  /**
   * The slot accepts value, which may be an in-place value or a predefined identifier.
   * For example: `123` in `println 123`.
   */
  Value = 'value',

  /**
   * The slot accepts address, which must be a predefined identifier.
   * For example: `x` in `x = 123`.
   */
  Address = 'address'
}
```

```typescript
/**
 * Info about what inputs are accepted by a slot.
 */
type SpxInputSlotAccept =
  | {
      /**
       * Input type accepted by the slot.
       */
      type:
        | SpxInputType.Integer
        | SpxInputType.Decimal
        | SpxInputType.String
        | SpxInputType.Boolean
        | SpxInputType.SpxDirection
        | SpxInputType.SpxColor
        | SpxInputType.SpxEffectKind
        | SpxInputType.SpxKey
        | SpxInputType.SpxPlayAction
        | SpxInputType.SpxSpecialObj
        | SpxInputType.SpxRotationStyle
        | SpxInputType.Unknown
    }
  | {
      /**
       * Input type accepted by the slot.
       */
      type: SpxInputType.SpxResourceName
      /**
       * Resource context.
       */
      resourceContext: SpxResourceContextURI
    }
```

```typescript
/**
 * The type of input for a slot.
 */
enum SpxInputType {
  /**
   * Integer number values.
   */
  Integer = 'integer',

  /**
   * Decimal number values.
   */
  Decimal = 'decimal',

  /**
   * String values.
   */
  String = 'string',

  /**
   * Boolean values.
   */
  Boolean = 'boolean',

  /**
   * Resource name (`SpriteName`, `SoundName`, etc.) in spx.
   */
  SpxResourceName = 'spx-resource-name',

  /**
   * Direction values in spx.
   */
  SpxDirection = 'spx-direction',

  /**
   * Color values in spx.
   */
  SpxColor = 'spx-color',

  /**
   * Effect kind values in spx.
   */
  SpxEffectKind = 'spx-effect-kind',

  /**
   * Keyboard key values in spx.
   */
  SpxKey = 'spx-key',

  /**
   * Sound playback action values in spx.
   */
  SpxPlayAction = 'spx-play-action',

  /**
   * Special object values in spx.
   */
  SpxSpecialObj = 'spx-special-obj',

  /**
   * Rotation style values in spx.
   */
  SpxRotationStyle = 'spx-rotation-style',

  /**
   * Unknown type.
   */
  Unknown = 'unknown'
}
```

```typescript
/**
 * Name for color constructors.
 */
type SpxInputTypeSpxColorConstructor = 'HSB' | 'HSBA'
```

```typescript
/**
 * Input value with type information.
 */
type SpxInputTypedValue =
  | { type: SpxInputType.Integer; value: number }
  | { type: SpxInputType.Decimal; value: number }
  | { type: SpxInputType.String; value: string }
  | { type: SpxInputType.Boolean; value: boolean }
  | { type: SpxInputType.SpxResourceName; value: ResourceURI }
  | { type: SpxInputType.SpxDirection; value: number }
  | {
      type: SpxInputType.SpxColor
      value: {
        /**
         * Constructor for color.
         */
        constructor: SpxInputTypeSpxColorConstructor
        /**
         * Arguments passed to the constructor.
         */
        args: number[]
      }
    }
  | { type: SpxInputType.SpxEffectKind; value: string }
  | { type: SpxInputType.SpxKey; value: string }
  | { type: SpxInputType.SpxPlayAction; value: string }
  | { type: SpxInputType.SpxSpecialObj; value: string }
  | { type: SpxInputType.SpxRotationStyle; value: string }
  | { type: SpxInputType.Unknown; value: void }
```

```typescript
/**
 * URI of the resource context. Examples:
 * - `spx://resources/sprites`
 * - `spx://resources/sounds`
 * - `spx://resources/sprites/<sName>/costumes`
 */
type SpxResourceContextURI = string
```

```typescript
/**
 * Represents the current input in a slot.
 */
type SpxInput<T extends SpxInputTypedValue = SpxInputTypedValue> =
  | {
      /**
       * In-place value
       * For example: `"hello world"`, `123`, `true`, spx `Left`, spx `HSB(0,0,0)`
       */
      kind: SpxInputKind.InPlace

      /**
       * Type of the input.
       */
      type: T['type']

      /**
       * In-place value.
       */
      value: T['value']
    }
  | {
      /**
       * (Reference to) user predefined identifier
       * For example: var `costume1`, const `name2`, field `num3`
       */
      kind: SpxInputKind.Predefined

      /**
       * Type of the input.
       */
      type: T['type']

      /**
       * Name for user predefined identifer.
       */
      name: string
    }
```

```typescript
/**
 * The kind of input.
 */
enum SpxInputKind {
  /**
   * In-place value
   * For example: `"hello world"`, `123`, `true`, spx `Left`, spx `HSB(0,0,0)`
   */
  InPlace = 'in-place',

  /**
   * (Reference to) user predefined identifier
   * For example: var `costume1`, const `name2`, field `num3`
   */
  Predefined = 'predefined'
}
```

## Other JSON structures

### Document link data types

```typescript
/**
 * The data of an spx resource reference DocumentLink.
 */
interface SpxResourceRefDocumentLinkData {
  /**
   * The kind of the spx resource reference.
   */
  kind: SpxResourceRefKind
}
```

```typescript
/**
 * The kind of the spx resource reference.
 *
 * - stringLiteral: String literal as a resource-reference, e.g., `play "explosion"`
 * - autoBinding: Auto-binding variable as a resource-reference, e.g., `var explosion Sound`
 * - autoBindingReference: Reference for auto-binding variable as a resource-reference, e.g., `play explosion`
 * - constantReference: Reference for constant as a resource-reference, e.g., `play EXPLOSION` (`EXPLOSION` is a constant)
 */
type SpxResourceRefKind = 'stringLiteral' | 'autoBinding' | 'autoBindingReference' | 'constantReference'
```

### Completion item data types

```typescript
/**
 * The data of a completion item.
 */
interface CompletionItemData {
  /**
   * The corresponding definition of the completion item.
   */
  definition?: SpxDefinitionIdentifier
}
```
