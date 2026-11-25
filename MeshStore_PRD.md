# Product Requirements Document (PRD)
## MeshStore — Distributed Mesh Storage & Seamless File Transfer

**Version:** 1.0  
**Status:** Product specification / Development blueprint  
**Product type:** Distributed storage system + desktop frontend  
**Initial target platform:** Windows  
**Backend:** Go  
**Frontend:** React + TypeScript  
**Communication:** TCP-based peer-to-peer networking  
**Storage architecture:** Distributed chunk storage with replication  
**Project stage:** Existing prototype moving toward a functional multi-device storage pool

---

## 1. Executive Summary

MeshStore is a decentralized storage application that allows users to combine the available storage capacity of multiple computers into one logical storage pool.

Instead of manually transferring files between computers or relying on a centralized cloud provider, users can connect their devices to a private mesh network. Each device contributes a configurable amount of disk space, and the application distributes file chunks across participating nodes.

Users interact with MeshStore through a modern graphical interface that allows them to:

- Add computers to their storage mesh.
- View the total storage capacity of the network.
- Upload, download, and transfer files between devices.
- Browse files through one shared namespace.
- Monitor device availability, storage usage, and transfer progress.
- Configure storage allocation and redundancy.
- Recover files from available replicas when a device becomes unavailable.
- Manage files without manually selecting which physical computer stores each file.

The core principle is to make distributed storage feel like using a single shared drive.

**Product vision:** Turn the combined storage of multiple personal computers into a private, resilient, and easy-to-use storage network.

---

## 2. Problem Statement

Users often have multiple computers, laptops, or storage devices with unused disk capacity. However, their storage remains fragmented across individual devices.

Current problems include:

1. Files must be manually copied between computers.
2. Available disk space on other devices is difficult to use transparently.
3. There is no unified view of files stored across the network.
4. Transferring large files can require repeated manual intervention.
5. A device becoming unavailable can make its files inaccessible.
6. Centralized cloud storage can involve recurring costs, storage limits, and privacy concerns.
7. Managing multiple network shares is inconvenient for nontechnical users.

MeshStore addresses these issues by providing a distributed storage layer with automatic placement, a shared file index, and a unified user interface.

### 2.1 Proposed solution

A user installs MeshStore on multiple devices and joins them to the same trusted mesh.

The system discovers or connects to authorized peers, tracks their available capacity, distributes file chunks, and maintains metadata describing where the chunks are stored.

The frontend presents the mesh as one logical storage system while the backend handles networking, storage placement, transfers, and recovery.

---

## 3. Product Goals and Non-Goals

### 3.1 Primary goals

- Combine the usable storage capacity of participating devices.
- Support direct peer-to-peer data transfers.
- Provide one shared file namespace.
- Distribute files automatically across nodes.
- Provide a simple frontend for file operations.
- Resume interrupted transfers where possible.
- Support node disconnections and subsequent recovery.
- Protect stored data and network communications.
- Run continuously as a background service when configured.
- Support an initial Windows-based deployment.

### 3.2 Non-goals for the initial release

The MVP will not attempt to provide:

- A replacement for enterprise-grade distributed databases.
- Automatic operation across arbitrary networks without connectivity configuration.
- Full erasure coding and advanced data placement from day one.
- Public, anonymous file-sharing networks.
- A complete cloud-sync replacement with collaborative editing.
- Guaranteed access to data when all nodes holding that data are offline.
- A fully POSIX-compatible network filesystem.
- Automatic access to every device's disk without user permission.

---

## 4. Target Users

### 4.1 Primary users

**Multi-device individuals**

Users who own two or more computers and want to share storage between them.

**Developers and technical users**

Users who want a private distributed storage environment for project files, backups, datasets, and media.

**Small teams and labs**

Groups that want to pool unused local storage across several computers without immediately purchasing a dedicated storage server.

### 4.2 Example use cases

- Store large datasets across several laptops and desktops.
- Transfer project files between a home computer and a workstation.
- Keep redundant copies of important documents.
- Access a shared library of videos, images, and development assets.
- Use spare storage on a secondary computer instead of buying another drive.

---

## 5. Core Product Architecture

The application consists of four major components.

### 5.1 Node agent

A lightweight Go process running on every participating device.

Responsibilities:

- Start and stop the node.
- Manage a persistent node identity.
- Expose a network listener.
- Connect to trusted peers.
- Reserve and manage local storage.
- Receive and serve chunks.
- Report health and available capacity.
- Validate incoming requests.
- Verify data integrity.
- Perform repair and replication tasks.

Each node must have its own configuration and storage directory.

Example configuration:

```yaml
node:
  listen_address: "0.0.0.0:7000"
  data_directory: "D:/MeshStore/data"
  max_storage_gb: 100

mesh:
  network_id: "home-mesh"
  peers:
    - "192.168.1.20:7000"

storage:
  chunk_size_mb: 4
  replication_factor: 2
```

This is an illustrative configuration; the exact format and defaults should be finalized during implementation.

### 5.2 Mesh coordinator

The coordinator maintains the metadata required to operate the mesh.

For the MVP, use a designated metadata coordinator with a persistent local database. This can initially run on one of the participating computers.

Responsibilities:

- Maintain the registered node list.
- Track file metadata and chunk locations.
- Allocate new chunks to suitable nodes.
- Coordinate replication.
- Record file upload and deletion operations.
- Detect stale node reports.
- Schedule recovery and repair.
- Resolve concurrent metadata changes.

**Important architectural distinction:** A metadata coordinator does not need to store all file contents. File chunks remain distributed across participating devices.

The MVP may use one coordinator for simplicity. A future release can introduce replicated metadata and leader election to eliminate this single point of failure.

### 5.3 Storage engine

The storage engine manages physical files and logical file objects.

Responsibilities:

- Split files into chunks.
- Generate cryptographic hashes.
- Store chunks on selected nodes.
- Reconstruct files from chunks.
- Verify data integrity.
- Maintain replica information.
- Remove unreferenced chunks safely.
- Track disk usage and storage quotas.

Each chunk receives a stable identifier based on its content hash or another collision-resistant identifier.

Example:

```text
File: project.zip

Logical file
    ├── Chunk A → Node 1
    ├── Chunk B → Node 2
    ├── Chunk C → Node 3
    └── Chunk D → Node 1
```

The file metadata records the original filename, size, chunk order, chunk hashes, and storage locations.

### 5.4 Frontend application

The frontend is the primary user interface for the storage mesh.

**Recommended stack:**

- React
- TypeScript
- Tailwind CSS
- A lightweight desktop shell such as Tauri
- Go backend exposed through a local API or controlled desktop IPC
- WebSockets or server-sent events for live updates

The interface should provide a polished desktop experience rather than requiring users to operate the system exclusively through a terminal.

A local web dashboard can be supported as an alternative, but the initial product should prioritize a reliable desktop experience.

---

## 6. Functional Requirements

### FR-01: Node registration and identity

Every device must have a unique, persistent identity.

The system must:

- Generate an identity during initial setup.
- Store identity credentials securely.
- Register the node with the mesh.
- Authenticate peers before allowing storage operations.
- Allow an administrator to approve or revoke a node.
- Preserve identity across restarts.
- Prevent an unauthorized node from joining merely by knowing an IP address.

**Acceptance criteria**

- Restarting a node preserves its identity.
- Authorized peers can reconnect after a restart.
- Revoked peers cannot perform new authenticated operations.

### FR-02: Device discovery and connectivity

Users must be able to add devices to the mesh.

The MVP should support:

- Manual peer addition using an IP address and port.
- Bootstrap-node configuration.
- Reconnection after temporary disconnection.
- Periodic peer health checks.
- Visible online/offline status.
- Clear errors for unreachable peers.

Automatic discovery on a local network may be added after manual joining works reliably.

**Acceptance criteria**

- A user can join two devices using their peer addresses.
- Each node reports its connection status.
- A temporary network interruption does not permanently invalidate the node.

### FR-03: Shared storage namespace

The mesh must provide a single logical namespace for files.

Users should be able to browse files without knowing which device holds each chunk.

Each file record must contain:

- Unique file identifier.
- Filename and logical path.
- File size.
- Creation and modification metadata where available.
- Chunk manifest.
- Integrity hashes.
- Replica locations.
- Upload status.
- Ownership and access permissions.

The initial release can provide a virtual file browser rather than mounting a native filesystem drive.

**Acceptance criteria**

- A file uploaded from one authorized device appears in the shared browser.
- Its metadata remains available after a node restart.
- Users can retrieve the file without manually selecting its storage nodes.

### FR-04: File upload and chunking

The system must accept files of different sizes and divide them into manageable chunks.

Requirements:

- Stream large files instead of loading them entirely into RAM.
- Calculate a cryptographic hash for each chunk.
- Record the chunk sequence and total file size.
- Show upload progress in the frontend.
- Handle disk-full and interrupted-upload errors.
- Verify uploaded data before marking the file complete.
- Avoid exposing incomplete files as fully uploaded.

The initial chunk size should be configurable, with a default in the low-megabyte range. Performance testing will determine the final value.

**Acceptance criteria**

- Small and large files upload successfully.
- Large files do not require equivalent amounts of free RAM.
- An interrupted upload is either safely resumed or clearly marked as incomplete.
- A file is published to the shared namespace only after its required chunks and metadata are committed.

### FR-05: Automatic storage allocation

The mesh must choose suitable nodes for incoming chunks.

The placement algorithm should consider:

- Available disk capacity.
- Configured storage limits.
- Node online status.
- Network connectivity.
- Replication requirements.
- Node health.
- Existing chunk placement.

For the MVP, a straightforward capacity-aware placement algorithm is sufficient.

Example:

1. Identify online nodes with enough reserved capacity.
2. Exclude nodes that cannot satisfy the replication policy.
3. Rank eligible nodes by available capacity and basic health.
4. Write chunks to selected nodes.
5. Confirm successful writes.
6. Commit the updated file manifest.

**Acceptance criteria**

- Users do not need to select a physical storage device for every file.
- Full or unavailable nodes are excluded from new placements.
- Failed writes do not result in falsely successful uploads.

### FR-06: Replication and data availability

The system must support replicated chunks to reduce the impact of device failures.

For the MVP:

- Support a configurable replication factor.
- Default to two copies when at least two suitable nodes are available.
- Prefer different physical devices for replicas.
- Track the number and locations of healthy copies.
- Warn users when redundancy requirements cannot be met.

If only one suitable node exists, the system must clearly report reduced protection rather than promising redundancy.

**Acceptance criteria**

- Replicated chunks can be retrieved from another available node.
- Loss of one replica does not corrupt a file when another valid replica exists.
- Missing replicas are detected and scheduled for repair when capacity is available.

### FR-07: File download and reconstruction

Users must be able to download a file from the shared namespace.

Requirements:

- Retrieve the required chunks from available peers.
- Verify each chunk against its recorded hash.
- Reassemble chunks in the correct order.
- Stream reconstructed content to disk.
- Report download progress and errors.
- Support cancellation.
- Avoid overwriting an existing file without an explicit user decision.

**Acceptance criteria**

- The reconstructed file matches the original byte-for-byte.
- Missing or corrupted chunks generate an explicit error.
- Users can download files without knowing their physical locations.

### FR-08: Seamless file transfer between devices

Seamless transfer is a core product requirement, not an optional feature.

The frontend must provide a unified transfer interface.

Users should be able to:

- Drag and drop files into the application.
- Upload files from the current device into the mesh.
- Download files to the current device.
- Select one or multiple files.
- Transfer files between authorized devices.
- See real-time transfer progress.
- Pause and resume supported transfers.
- Cancel transfers.
- Retry failed transfers.
- View transfer history.
- Open the destination folder after a download completes.

Where a source file is already stored in the mesh, the system should reuse its verified chunks instead of unnecessarily uploading duplicate content.

Where possible, the system should transfer chunks directly between source and destination nodes. The metadata coordinator should coordinate the operation without unnecessarily routing file contents through itself.

**Acceptance criteria**

- A user can transfer a file without opening a terminal.
- Progress and errors are visible.
- A network interruption does not silently produce a partial file that appears complete.
- The frontend clearly distinguishes queued, active, paused, completed, and failed transfers.

### FR-09: File deletion

Deleting a logical file must not immediately remove shared chunks that other files still reference.

The system must:

- Remove the file's logical metadata.
- Track chunk references or use an equivalent safe lifecycle mechanism.
- Delete physical chunks only when they are no longer referenced and retention rules permit it.
- Apply deletion across replicas.
- Report pending cleanup operations.

**Acceptance criteria**

- Deleting one file does not corrupt another file sharing the same chunks.
- Failed cleanup operations can be retried safely.
- Deletion status is visible to the user.

### FR-10: Node offline and recovery

The system must handle nodes that disconnect unexpectedly.

Requirements:

- Mark a node offline after a configurable failure-detection period.
- Stop scheduling new writes to unavailable nodes.
- Continue serving data from healthy replicas.
- Reconnect automatically when possible.
- Detect missing replicas after recovery.
- Schedule replication repair.
- Display degraded storage health in the frontend.

A node being offline does not mean its physical data has disappeared. The metadata layer must distinguish unavailable data from confirmed missing or corrupted data.

### FR-11: Integrity and consistency

The system must detect incomplete or corrupted storage.

Requirements:

- Hash chunks using a modern cryptographic hash.
- Verify chunks on upload and retrieval.
- Record write completion and metadata commitment separately.
- Use safe temporary files and atomic publication where supported.
- Make retries idempotent.
- Prevent concurrent operations from creating conflicting manifests.
- Provide a periodic integrity-check mechanism.

### FR-12: Storage limits

Each node owner must be able to decide how much storage to contribute.

Requirements:

- Set a maximum storage allocation.
- Display total, used, and available contributed capacity.
- Avoid using unrelated disk space beyond the configured limit.
- Reserve space for operating-system and filesystem needs.
- Prevent new allocations when capacity is exhausted.
- Report the difference between raw capacity and usable protected capacity.

---

## 7. Frontend Product Requirements

### 7.1 Design principles

The interface must feel like a modern file-management application.

Design goals:

- Clean and minimal layout.
- Clear navigation.
- Drag-and-drop interactions.
- Live status updates.
- Easy-to-understand storage indicators.
- Responsive layouts.
- Useful error messages rather than raw Go errors.
- Dark and light themes.
- Accessible controls and readable text.

The frontend must not require users to understand chunks, hashes, peer protocols, or network topology to perform ordinary file operations.

### 7.2 Main screens

#### Screen A: Dashboard

Display:

- Total contributed storage.
- Total used storage.
- Available storage.
- Number of online devices.
- Number of offline devices.
- Active transfers.
- Storage health.
- Recent file activity.

Example:

```text
MeshStore Dashboard

Storage used       420 GB
Available          680 GB
Contributed total  1.1 TB

Devices online     3 / 4
Active transfers   2
Storage health     Degraded
```

These values are illustrative, not actual capacity calculations.

The dashboard should explain when available storage is estimated or when the mesh is operating below its desired redundancy level.

#### Screen B: Shared Files

Provide a familiar file-browser interface with:

- Grid and list views.
- Search.
- Sorting by name, size, and modification date.
- Folder navigation.
- File previews where supported.
- Multi-select.
- Upload, download, rename, and delete actions.
- File details and availability status.

Users should not need to know which node holds a particular chunk.

#### Screen C: Transfer Center

The Transfer Center should provide:

- Upload queue.
- Download queue.
- Device-to-device transfers.
- Progress percentage.
- Transfer speed.
- Estimated remaining time.
- Current transfer state.
- Pause, resume, cancel, and retry actions where supported.
- Failure explanations.
- Transfer history.

Transfers should remain visible when users navigate to other screens.

#### Screen D: Devices

Show each node with:

- Device name.
- Persistent node ID.
- IP address and port.
- Online/offline status.
- Total contributed capacity.
- Used and available capacity.
- Last successful contact.
- Current transfer activity.
- Storage health.
- Remove or revoke actions for authorized administrators.

Adding a device should be guided by a simple flow:

1. Install or start MeshStore on the second device.
2. Generate or retrieve its pairing information.
3. Approve the device.
4. Connect to the mesh.
5. Confirm that the device is online.

The application must not automatically trust arbitrary peers.

#### Screen E: Storage Settings

Allow users to configure:

- Storage contribution per device.
- Default replication factor.
- Chunk-size policy.
- Upload and download bandwidth limits.
- Automatic reconnect.
- Repair scheduling.
- Download destination.
- Notifications.
- Theme.
- Security and device approvals.

Advanced settings should be separated from basic settings.

#### Screen F: File Details

For a selected file, show:

- Name and size.
- Upload date.
- Integrity status.
- Availability status.
- Number of healthy copies.
- Nodes storing its chunks.
- Download action.
- Delete action.
- Repair status if relevant.

Physical placement details should be available as optional advanced information.

---

## 8. Recommended Technical Architecture

### 8.1 System overview

```text
                ┌─────────────────────────┐
                │     Desktop Frontend    │
                │    React + TypeScript   │
                └────────────┬────────────┘
                             │
                    Local API / IPC
                             │
                ┌────────────▼────────────┐
                │       Go Node Agent     │
                │                         │
                │  API and Control Layer  │
                │  Peer Transport         │
                │  Transfer Manager       │
                │  Local Storage Engine   │
                └────────────┬────────────┘
                             │
                     Authenticated P2P
                             │
              ┌──────────────┼──────────────┐
              │              │              │
        ┌─────▼─────┐  ┌─────▼─────┐  ┌─────▼─────┐
        │  Node A   │  │  Node B   │  │  Node C   │
        │  Storage  │  │  Storage  │  │  Storage  │
        └───────────┘  └───────────┘  └───────────┘
              │              │              │
              └──────────────┼──────────────┘
                             │
                  Shared Metadata Index
                   (MVP coordinator)
```

The frontend communicates with the local node agent. The node agent communicates with authorized peers. The metadata coordinator maintains the authoritative file and placement index for the MVP.

### 8.2 Backend modules

Recommended Go package boundaries:

```text
cmd/
  meshstore/
internal/
  config/
  identity/
  api/
  node/
  transport/
  peer/
  metadata/
  storage/
  chunking/
  placement/
  replication/
  transfer/
  recovery/
  security/
  telemetry/
```

The existing `main.go` demo should gradually become a command-line entry point. Production behavior should live in independently testable packages rather than being tied to three hardcoded server instances.

### 8.3 API requirements

The backend should expose a versioned local API.

Illustrative endpoints:

| Method | Endpoint | Purpose |
|---|---|---|
| GET | `/api/v1/status` | Current node and mesh status |
| GET | `/api/v1/nodes` | List authorized nodes |
| POST | `/api/v1/nodes/pair` | Start a pairing flow |
| GET | `/api/v1/files` | Browse shared files |
| POST | `/api/v1/files/upload` | Upload a file |
| GET | `/api/v1/files/{id}` | Read file metadata |
| GET | `/api/v1/files/{id}/download` | Download a file |
| DELETE | `/api/v1/files/{id}` | Delete a logical file |
| GET | `/api/v1/transfers` | List transfers |
| POST | `/api/v1/transfers/{id}/pause` | Pause a supported transfer |
| POST | `/api/v1/transfers/{id}/resume` | Resume a supported transfer |
| DELETE | `/api/v1/transfers/{id}` | Cancel a transfer |
| GET | `/api/v1/storage` | Read capacity and health |
| PATCH | `/api/v1/settings/storage` | Update storage settings |

These endpoints are proposed interfaces, not existing functionality.

The local API must use authentication or a restricted local IPC channel. A local API should not be exposed to the public network without an explicit, secure configuration.

### 8.4 Metadata model

A relational database such as SQLite is suitable for an initial single-coordinator implementation.

Suggested logical entities:

**Node**
- `node_id`
- `display_name`
- `public_key`
- `last_seen`
- `status`
- `capacity_bytes`
- `used_bytes`
- `storage_root`

**File**
- `file_id`
- `logical_path`
- `file_name`
- `size_bytes`
- `content_hash`
- `status`
- `created_at`
- `updated_at`

**Chunk**
- `chunk_id`
- `content_hash`
- `size_bytes`
- `reference_count` or equivalent lifecycle information

**ChunkLocation**
- `chunk_id`
- `node_id`
- `health_status`
- `last_verified_at`

**FileChunk**
- `file_id`
- `chunk_index`
- `chunk_id`

**Transfer**
- `transfer_id`
- `file_id`
- `source_node`
- `destination_node`
- `bytes_transferred`
- `state`
- `last_checkpoint`

**OperationLog**
- `operation_id`
- `operation_type`
- `status`
- `created_at`

Database migrations and crash-safe transaction boundaries must be implemented from the beginning.

---

## 9. Security and Privacy Requirements

Security is a release requirement because participating devices may contain private files.

The system must support:

1. Authenticated peer identities.
2. Encrypted network transport.
3. Secure device pairing and revocation.
4. Local API authentication.
5. Strict path validation.
6. Storage-directory isolation.
7. Protection against unauthorized file access.
8. Safe handling of malformed network messages.
9. Secure credential storage.
10. Audit logs for sensitive administrative operations.

Stored file encryption should be designed separately from transport encryption. Encryption in transit alone does not protect data on a stolen or compromised storage device.

The design should also specify how encryption keys are recovered after restarts or device loss. A new random encryption key on every process start must not make previously stored data unreadable.

For an initial private mesh, use authenticated TLS or another well-reviewed secure transport design rather than implementing custom cryptography.

---

## 10. Performance and Reliability Requirements

The MVP must establish measurable performance baselines rather than promising a specific throughput before testing.

### 10.1 Performance targets

Initial engineering targets on a healthy local network:

- File listing: under 500 ms for a typical page of 100 entries.
- Dashboard refresh: under 1 second after receiving updated node statistics.
- Transfer progress updates: at least once per second.
- Small-file operations: avoid excessive per-file protocol overhead.
- Large-file operations: stream data without buffering the entire file in RAM.
- Reconnection: automatically retry within a configurable interval.
- Integrity verification: detect corrupted chunks before reporting successful reconstruction.

These are initial targets and should be measured on documented test hardware.

### 10.2 Reliability requirements

- A failed node must not cause a healthy replica to be deleted.
- An incomplete upload must not be published as complete.
- Metadata updates must survive ordinary process restarts.
- Retries must not create duplicate logical files unintentionally.
- Disk-full conditions must return actionable errors.
- A failed repair operation must be retried or surfaced to the user.
- All storage operations must remain within configured storage roots.

---

## 11. Storage Capacity and Redundancy

The frontend must distinguish raw capacity from usable protected capacity.

With replication factor `r`, the theoretical maximum usable capacity is approximately:

\[
C_{\text{usable}} \approx \frac{C_{\text{raw}}}{r}
\]

This is a simplified estimate before filesystem overhead, metadata, unequal node capacities, and placement constraints.

For example, if four devices contribute 500 GB each, the raw capacity is 2 TB. With two complete copies of each chunk, the theoretical usable capacity is approximately 1 TB.

Actual usable capacity can be lower because devices have different available space and redundancy requires replicas on distinct devices.

### Initial policy

- Use replicated storage for the MVP.
- Default to two copies when the mesh can support them.
- Warn users when a file has fewer healthy copies than requested.
- Avoid placing all copies on the same physical device.
- Add erasure coding only after placement, integrity, recovery, and transfer behavior are stable.

Erasure coding is a future optimization, not a prerequisite for the first usable version.

---

## 12. Failure Scenarios and Expected Behavior

| Scenario | Required behavior |
|---|---|
| A node disconnects | Mark it offline and stop assigning new work to it |
| A node restarts | Reconnect using its persistent identity |
| A chunk is corrupted | Reject it and retrieve a healthy replica if available |
| A device runs out of space | Stop new allocation and report the issue |
| A transfer is interrupted | Resume from a verified checkpoint where supported |
| All replicas are unavailable | Report that the file cannot currently be retrieved |
| Metadata coordinator fails | Preserve local chunks and recover coordinator metadata from persistent storage or backup |
| Two users modify the same logical path | Apply a documented conflict policy |
| A user deletes a file | Remove logical references safely and clean up unreferenced chunks |
| An unauthorized node attempts to join | Reject it until explicitly authorized |

A future high-availability version should replicate metadata and provide coordinator failover.

---

## 13. MVP Scope

The first usable release should focus on a small set of complete workflows.

### MVP features

- Configurable Go node process.
- Persistent node identity.
- Manual peer configuration.
- Authenticated peer communication.
- Three or more nodes running as independent processes, including on separate computers.
- Shared metadata index.
- Chunk-based file storage.
- Capacity-aware placement.
- Two-copy replication where supported.
- File upload and download.
- Shared file browser.
- Desktop frontend.
- Transfer progress and error reporting.
- Persistent transfer state.
- Node health dashboard.
- Basic repair after a replica becomes unavailable.
- Local storage quotas.
- Safe file deletion.
- Basic automated tests.

### Features deferred until after the MVP

- Erasure coding.
- Fully decentralized metadata consensus.
- Automatic internet-wide NAT traversal.
- Native drive mounting.
- Cross-platform release packaging beyond the initial target.
- Advanced deduplication across the entire mesh.
- Version history and snapshots.
- Public sharing links.
- Mobile applications.
- Multi-user organizational administration.

---

## 14. Development Roadmap

### Phase 1 — Stabilize the existing prototype

**Goal:** Turn the current demo into a reliable node program.

Tasks:
- Separate server startup from storage operations.
- Replace hardcoded node addresses with configuration.
- Correct storage-root construction and path handling.
- Establish persistent identity.
- Add startup and shutdown handling.
- Add basic tests for read, write, and delete.
- Ensure files are closed correctly and temporary files are cleaned up safely.

Deliverable: One independently configurable Go node.

### Phase 2 — Build the storage protocol

**Goal:** Enable independent nodes to exchange chunks.

Tasks:
- Define authenticated peer messages.
- Implement chunk identifiers and manifests.
- Implement chunk upload and retrieval.
- Add write acknowledgements.
- Add hash verification.
- Implement safe retries.
- Add a configurable peer list.

Deliverable: Multiple devices can exchange verified chunks.

### Phase 3 — Implement the shared index

**Goal:** Present distributed files as one logical collection.

Tasks:
- Add the persistent metadata database.
- Implement file and chunk records.
- Track chunk locations.
- Implement upload commit semantics.
- Add node heartbeats.
- Implement capacity reporting.
- Add initial placement logic.

Deliverable: A file uploaded through one node is discoverable through the shared index.

### Phase 4 — Add replication and recovery

**Goal:** Improve availability when a device disconnects.

Tasks:
- Replicate chunks across distinct nodes.
- Track healthy replica counts.
- Detect unavailable nodes.
- Add repair scheduling.
- Test disk-full and node-loss scenarios.
- Prevent unsafe deletion of referenced chunks.

Deliverable: Files remain retrievable when a single storage node becomes unavailable, provided another healthy copy exists.

### Phase 5 — Build the frontend

**Goal:** Eliminate the need for terminal-based file management.

Tasks:
- Create the dashboard.
- Build the shared file browser.
- Add device management and pairing.
- Add upload and download dialogs.
- Build the Transfer Center.
- Show progress through WebSockets or server-sent events.
- Add error notifications.
- Implement theme and settings.
- Package the frontend with the Go node agent.

Deliverable: Users can manage files and devices through a desktop application.

### Phase 6 — Reliability and release

**Goal:** Prepare the system for regular personal use.

Tasks:
- Test multiple real devices.
- Test large files and interrupted transfers.
- Verify persistent metadata after restarts.
- Test concurrent operations.
- Conduct security reviews.
- Measure throughput and memory use.
- Add structured logs and diagnostics.
- Document installation and backup procedures.
- Package a reproducible Windows build.

Deliverable: A documented, testable MVP release.

---

## 15. Testing Strategy

### Unit tests

Cover:
- Chunk creation and hashing.
- File manifest generation.
- Path validation.
- Placement decisions.
- Metadata updates.
- Replica accounting.
- Transfer state transitions.

### Integration tests

Cover:
- Upload and download between two nodes.
- File reconstruction from multiple chunks.
- Replication across three nodes.
- Restarting a node without losing identity.
- Reconnecting after a network interruption.
- Deleting files without deleting shared chunks.
- Coordinator restart and metadata recovery.

### End-to-end tests

Cover:
- Adding a device through the frontend.
- Uploading a file with drag and drop.
- Viewing the file on another device.
- Downloading and verifying the file.
- Interrupting and resuming a transfer.
- Disconnecting a replica node.
- Restoring the node and checking recovery status.

The test suite should include binary files, empty files, large files, unusual filenames, insufficient disk space, and corrupted chunks.

---

## 16. Success Metrics

The MVP will be considered successful when:

- Three independent nodes can join the same mesh.
- Each node maintains its own local storage directory.
- A file can be uploaded through one node and retrieved through another.
- Files are stored in chunks with verified integrity.
- The configured replication policy is enforced when sufficient capacity exists.
- A healthy replica can serve a file when another node goes offline.
- The frontend accurately reports storage and transfer status.
- Users can perform common file operations without terminal commands.
- Interrupted transfers do not silently produce incomplete files.
- A node restart preserves identity and stored-data accessibility.
- The application passes the agreed integration and end-to-end tests.

---

## 17. Key Technical Decisions

The following decisions should be treated as the default implementation direction.

| Decision | MVP choice |
|---|---|
| Backend language | Go |
| Frontend | React + TypeScript |
| Desktop delivery | Tauri or an equivalent lightweight shell |
| Initial operating system | Windows |
| Node topology | Independent peer nodes with a designated metadata coordinator |
| Metadata storage | SQLite for the initial coordinator |
| File distribution | Content-addressed chunks |
| Initial redundancy | Replication |
| Default replication factor | Two copies when supported |
| Device joining | Manual address + authenticated pairing |
| Transfer progress | WebSockets or server-sent events |
| Network protection | Authenticated encrypted transport |
| Storage model | Shared logical namespace over distributed physical storage |
| Initial interface | Desktop file browser and Transfer Center |

These are recommendations for the first release, not permanent architectural constraints.

---

## 18. Risks and Mitigations

**Risk: Metadata coordinator failure**

Mitigation: Persist metadata, implement backups and recovery, and plan for replicated metadata in a later release.

**Risk: Data loss from deleting chunks prematurely**

Mitigation: Track references, use safe cleanup procedures, and verify that remaining replicas are healthy before destructive operations.

**Risk: Devices have different capacities and network speeds**

Mitigation: Use capacity-aware placement and expose per-node health and capacity.

**Risk: Windows file-locking behavior**

Mitigation: Test temporary-file publication, cleanup, and deletion under Windows. Avoid removing files while open handles remain active.

**Risk: Network access across different networks**

Mitigation: Start with a trusted local network and explicitly configured reachable peers. Treat NAT traversal and relay services as a later networking milestone.

**Risk: Security weaknesses**

Mitigation: Use authenticated peers, encrypted transport, secure key persistence, strict API authorization, and adversarial protocol tests.

**Risk: Unreliable progress reporting**

Mitigation: Make transfer state persistent, distinguish bytes sent from bytes committed, and report completion only after verification.

---

## 19. Final Product Definition

MeshStore is not merely a program that copies files between computers. It is a distributed storage system with a shared namespace, storage placement, replication, recovery, and a graphical interface.

The finished product should let a user:

1. Install MeshStore on several devices.
2. Approve those devices into a private mesh.
3. Allocate storage on each device.
4. See the combined storage pool in one dashboard.
5. Drag and drop files into a shared file browser.
6. Let the system decide where chunks should live.
7. Download files from any authorized device.
8. Monitor transfers and storage health.
9. Recover data from healthy replicas when a device disconnects.
10. Manage the entire system without needing to understand the underlying networking implementation.

**The first development priority is to stabilize the existing Go node and implement persistent metadata plus chunk placement. The frontend should then connect to well-defined backend APIs, so the user experience and storage engine can evolve independently.**

This approach keeps the first release achievable while establishing a foundation for a more resilient, capacity-efficient, multi-device mesh storage platform.
