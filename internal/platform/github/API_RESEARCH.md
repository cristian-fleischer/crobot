# GitHub REST API Research for CRoBot Platform Adapter

> Research date: 2026-03-18
> API version: `2022-11-28` (use `X-GitHub-Api-Version` header)
> Base URL: `https://api.github.com`

---

## Table of Contents

1. [Authentication](#1-authentication)
2. [Common Headers](#2-common-headers)
3. [Rate Limiting](#3-rate-limiting)
4. [Pagination](#4-pagination)
5. [PR Metadata](#5-pr-metadata)
6. [Changed Files](#6-changed-files)
7. [Raw Diff](#7-raw-diff)
8. [File Content at Commit](#8-file-content-at-commit)
9. [PR Review Comments](#9-pr-review-comments)
10. [Mapping to CRoBot Platform Interface](#10-mapping-to-crobot-platform-interface)

---

## 1. Authentication

### Bearer Token (Recommended)

```
Authorization: Bearer <token>
```

Supports:
- **Personal Access Tokens (classic)**: scope `repo` for private repos
- **Fine-grained PATs**: select specific repos and permissions (recommended for security)
- **GitHub App installation tokens**: for integrations
- **`GITHUB_TOKEN`**: for GitHub Actions workflows

### Rate limits by auth type

| Auth type                  | Requests/hour |
|----------------------------|---------------|
| Unauthenticated            | 60            |
| Personal access token      | 5,000         |
| GitHub App (Enterprise)    | 15,000        |
| `GITHUB_TOKEN` (Actions)   | 1,000         |

### Implementation Notes

- CRoBot should accept a single `token` field in config (no username needed, unlike Bitbucket)
- Token is passed via `Authorization: Bearer <token>` header
- Fine-grained PATs need **Pull Requests: Read/Write** and **Contents: Read** permissions

---

## 2. Common Headers

Every request MUST include:

```http
Accept: application/vnd.github+json
Authorization: Bearer <token>
User-Agent: CRoBot/<version>
X-GitHub-Api-Version: 2022-11-28
```

**Important**: GitHub rejects requests without a valid `User-Agent` header. Use `CRoBot/<version>`.

---

## 3. Rate Limiting

### Response Headers

Every response includes:

| Header                    | Description                                         |
|---------------------------|-----------------------------------------------------|
| `x-ratelimit-limit`      | Max requests per hour                               |
| `x-ratelimit-remaining`  | Remaining requests in current window                |
| `x-ratelimit-used`       | Requests made in current window                     |
| `x-ratelimit-reset`      | Unix epoch timestamp when window resets              |
| `x-ratelimit-resource`   | Rate limit resource bucket (e.g., `core`)           |

### Primary Rate Limit Exceeded

- HTTP status: `403`
- `x-ratelimit-remaining` will be `0`
- Wait until `x-ratelimit-reset` timestamp

### Secondary (Abuse) Rate Limit

- HTTP status: `429`
- `retry-after` header contains seconds to wait
- Triggered by: >100 concurrent requests, >900 points/minute (GET=1pt, POST/PUT/DELETE=5pts)
- Content creation limit: 80/minute, 500/hour

### Retry Strategy

```
if status == 429:
    wait = parse_int(response.Header("retry-after"))
    sleep(wait * time.Second)
elif status == 403 and ratelimit_remaining == 0:
    reset = parse_int(response.Header("x-ratelimit-reset"))
    wait = reset - time.Now().Unix()
    sleep(wait * time.Second)
else:
    exponential_backoff(attempt, base=1s, max=30s)
```

Recommended: up to 3 retries with exponential backoff (matching Bitbucket adapter pattern).

---

## 4. Pagination

### Link Header Format

```http
Link: <https://api.github.com/repos/owner/repo/pulls/1/files?page=2>; rel="next",
      <https://api.github.com/repos/owner/repo/pulls/1/files?page=5>; rel="last"
```

### Relation types

| rel       | Description                       |
|-----------|-----------------------------------|
| `next`    | Next page URL                     |
| `prev`    | Previous page URL                 |
| `first`   | First page URL                    |
| `last`    | Last page URL                     |

### Parsing

Parse with regex: `<([^>]+)>;\s*rel="([^"]+)"`

### Parameters

- `per_page`: results per page (max 100, default 30)
- `page`: 1-based page index

### Implementation Notes

- Always set `per_page=100` to minimize requests
- Follow `rel="next"` links until absent (no next = last page)
- Validate pagination URLs match `api.github.com` host (SSRF protection, matching Bitbucket adapter)

---

## 5. PR Metadata

### Endpoint

```
GET /repos/{owner}/{repo}/pulls/{pull_number}
```

### Response Shape (key fields for CRoBot)

```json
{
  "number": 42,
  "title": "Fix authentication bug",
  "body": "Description of changes...",
  "state": "open",
  "user": {
    "login": "octocat",
    "id": 1
  },
  "head": {
    "ref": "feature-branch",
    "sha": "abc123def456...",
    "repo": {
      "full_name": "owner/repo"
    }
  },
  "base": {
    "ref": "main",
    "sha": "789xyz...",
    "repo": {
      "full_name": "owner/repo"
    }
  },
  "merged": false,
  "draft": false,
  "additions": 10,
  "deletions": 5,
  "changed_files": 3,
  "comments": 2,
  "review_comments": 1,
  "html_url": "https://github.com/owner/repo/pull/42",
  "diff_url": "https://github.com/owner/repo/pull/42.diff",
  "created_at": "2026-01-01T00:00:00Z",
  "updated_at": "2026-01-02T00:00:00Z"
}
```

### Mapping to `platform.PRContext`

```go
PRContext{
    ID:           pr.Number,           // json:"number"
    Title:        pr.Title,            // json:"title"
    Description:  pr.Body,             // json:"body"
    Author:       pr.User.Login,       // json:"user" -> "login"
    SourceBranch: pr.Head.Ref,         // json:"head" -> "ref"
    TargetBranch: pr.Base.Ref,         // json:"base" -> "ref"
    State:        mapState(pr.State),  // "open" -> "OPEN", "closed" -> "CLOSED"/"MERGED"
    HeadCommit:   pr.Head.SHA,         // json:"head" -> "sha"
    BaseCommit:   pr.Base.SHA,         // json:"base" -> "sha"
}
```

**State mapping**: GitHub uses `"open"` / `"closed"`. Check `pr.Merged == true` to distinguish `"MERGED"` from `"CLOSED"`.

### Status Codes

| Code | Meaning         |
|------|-----------------|
| 200  | Success         |
| 304  | Not modified    |
| 404  | Not found       |
| 500  | Server error    |
| 503  | Service unavail |

---

## 6. Changed Files

### Endpoint

```
GET /repos/{owner}/{repo}/pulls/{pull_number}/files?per_page=100
```

**Important**: Maximum 3000 files returned. Paginated, default 30 per page.

### Response Shape

```json
[
  {
    "sha": "abc123",
    "filename": "src/auth.ts",
    "status": "modified",
    "additions": 5,
    "deletions": 2,
    "changes": 7,
    "blob_url": "https://github.com/...",
    "raw_url": "https://github.com/...",
    "contents_url": "https://api.github.com/repos/owner/repo/contents/src/auth.ts?ref=abc123",
    "patch": "@@ -10,6 +10,9 @@ ...\n context\n-old line\n+new line\n context",
    "previous_filename": "src/old-auth.ts"
  }
]
```

### File Status Values

| Status      | Description                              | CRoBot mapping |
|-------------|------------------------------------------|----------------|
| `added`     | New file                                 | `"added"`      |
| `removed`   | Deleted file                             | `"deleted"`    |
| `modified`  | Content changed                          | `"modified"`   |
| `renamed`   | Path changed (may include content changes)| `"renamed"`   |
| `copied`    | Copied from another file                 | `"added"`      |
| `changed`   | File type changed (e.g., regular -> symlink) | `"modified"` |
| `unchanged` | No changes (included in some contexts)   | skip           |

### Mapping to `platform.ChangedFile`

```go
ChangedFile{
    Path:    file.Filename,          // json:"filename"
    OldPath: file.PreviousFilename,  // json:"previous_filename" (only for renames)
    Status:  mapStatus(file.Status), // see table above
}
```

### Important Notes

- `previous_filename` is only present for `renamed` and `copied` status
- `patch` field may be absent for binary files or very large diffs
- Pagination required: follow Link header `rel="next"`

---

## 7. Raw Diff

### Approach: Use `Accept` header on PR endpoint

```
GET /repos/{owner}/{repo}/pulls/{pull_number}
Accept: application/vnd.github.diff
```

Returns the entire PR diff in unified diff format (same as `git diff`).

### Alternative: diff_url field

The PR metadata response includes `diff_url` (e.g., `https://github.com/owner/repo/pull/42.diff`) which can be fetched directly, but this may require authentication for private repos.

### Response Format

Plain text unified diff:

```diff
diff --git a/src/auth.ts b/src/auth.ts
index abc123..def456 100644
--- a/src/auth.ts
+++ b/src/auth.ts
@@ -10,6 +10,9 @@ function authenticate() {
   const token = getToken();
-  console.log(token);
+  logger.debug("Token received");
+  logger.info("Auth successful");
   return token;
 }
```

### Implementation Notes

- Parse using the same `parseDiff()` function as the Bitbucket adapter (unified diff format is identical)
- The diff response can be large; use the same 50MB body limit as Bitbucket adapter
- Content-Type will be `text/plain` or `application/x-diff`

---

## 8. File Content at Commit

### Endpoint

```
GET /repos/{owner}/{repo}/contents/{path}?ref={commit_sha}
```

### Response Shape (file)

```json
{
  "type": "file",
  "encoding": "base64",
  "size": 1234,
  "name": "auth.ts",
  "path": "src/auth.ts",
  "content": "aW1wb3J0IHsgLi4uIH0=\n...",
  "sha": "abc123",
  "url": "https://api.github.com/repos/owner/repo/contents/src/auth.ts?ref=abc123",
  "git_url": "https://api.github.com/repos/owner/repo/git/blobs/abc123",
  "html_url": "https://github.com/owner/repo/blob/abc123/src/auth.ts",
  "download_url": "https://raw.githubusercontent.com/owner/repo/abc123/src/auth.ts",
  "_links": {
    "self": "...",
    "git": "...",
    "html": "..."
  }
}
```

### Alternative: Raw content via media type

```
GET /repos/{owner}/{repo}/contents/{path}?ref={commit_sha}
Accept: application/vnd.github.raw+json
```

Returns raw file content directly (no base64 encoding). **Recommended** — simpler and avoids decoding.

### Size Limits

| Size      | Supported methods                |
|-----------|----------------------------------|
| ≤ 1 MB    | All media types                  |
| 1-100 MB  | Raw or object media types only   |
| > 100 MB  | Not supported (use Git Blobs API)|

### Implementation Notes

- Use `application/vnd.github.raw+json` Accept header to get raw content directly (avoids base64 decode)
- Pass commit SHA via `ref` query parameter
- For files > 1MB, must use raw media type
- Enforce 10MB body limit (matching Bitbucket adapter)

### Status Codes

| Code | Meaning                |
|------|------------------------|
| 200  | Success                |
| 302  | Redirect (follow it)   |
| 403  | Forbidden              |
| 404  | File/commit not found  |

---

## 9. PR Review Comments

### 9a. List Comments

```
GET /repos/{owner}/{repo}/pulls/{pull_number}/comments?per_page=100
```

#### Response Shape

```json
[
  {
    "id": 12345,
    "node_id": "MDI0OlB1bGxSZXF1ZXN0UmV2aWV3Q29tbWVudDEyMzQ1",
    "body": "Comment text here\n\n[//]: # \"crobot:fp=abc123\"",
    "path": "src/auth.ts",
    "line": 42,
    "side": "RIGHT",
    "start_line": null,
    "start_side": null,
    "commit_id": "abc123def456",
    "original_commit_id": "abc123def456",
    "original_line": 42,
    "diff_hunk": "@@ -10,6 +10,9 @@ ...",
    "user": {
      "login": "crobot[bot]",
      "id": 99999,
      "type": "Bot"
    },
    "author_association": "NONE",
    "created_at": "2026-01-01T00:00:00Z",
    "updated_at": "2026-01-01T00:00:00Z",
    "html_url": "https://github.com/owner/repo/pull/42#discussion_r12345",
    "pull_request_review_id": null,
    "in_reply_to_id": null,
    "subject_type": "line",
    "reactions": {
      "url": "...",
      "total_count": 0,
      "+1": 0, "-1": 0, "laugh": 0, "hooray": 0,
      "confused": 0, "heart": 0, "rocket": 0, "eyes": 0
    },
    "_links": {
      "self": { "href": "..." },
      "html": { "href": "..." },
      "pull_request": { "href": "..." }
    }
  }
]
```

#### Query Parameters

| Parameter   | Type   | Default    | Description                    |
|-------------|--------|------------|--------------------------------|
| `sort`      | string | `created`  | `created` or `updated`         |
| `direction` | string | `desc`     | `asc` or `desc`                |
| `since`     | string | —          | ISO 8601 timestamp filter      |
| `per_page`  | int    | 30         | Results per page (max 100)     |
| `page`      | int    | 1          | Page number                    |

#### Bot Detection

GitHub bot users have `user.type == "Bot"`. For PAT-based bot accounts, match on `user.login` instead (configurable).

#### Mapping to `platform.Comment`

```go
Comment{
    ID:          strconv.Itoa(comment.ID),   // int -> string
    Path:        comment.Path,                // json:"path"
    Line:        comment.Line,                // json:"line"
    Body:        comment.Body,                // json:"body"
    Author:      comment.User.Login,          // json:"user" -> "login"
    CreatedAt:   comment.CreatedAt,           // json:"created_at"
    IsBot:       comment.User.Type == "Bot",  // or match login
    Fingerprint: platform.ExtractFingerprint(comment.Body),
}
```

### 9b. Create Inline Comment

```
POST /repos/{owner}/{repo}/pulls/{pull_number}/comments
```

#### Request Body

```json
{
  "body": "Review comment text\n\n[//]: # \"crobot:fp=abc123\"",
  "commit_id": "abc123def456",
  "path": "src/auth.ts",
  "line": 42,
  "side": "RIGHT"
}
```

#### Required Fields

| Field       | Type   | Description                                          |
|-------------|--------|------------------------------------------------------|
| `body`      | string | Comment text (markdown)                              |
| `commit_id` | string | SHA of the head commit (from PR metadata `head.sha`) |
| `path`      | string | File path relative to repo root                      |
| `line`      | int    | Line number in the diff                              |
| `side`      | string | `RIGHT` (new/added) or `LEFT` (old/deleted)          |

#### Side Mapping (CRoBot -> GitHub)

| CRoBot `Side` | GitHub `side` | Meaning                  |
|----------------|---------------|--------------------------|
| `"new"`        | `"RIGHT"`     | Addition/new version     |
| `"old"`        | `"LEFT"`      | Deletion/old version     |

#### Response

- Status: `201 Created`
- Body: Full comment object (same shape as list response)

#### Important Notes

- `commit_id` MUST be the head commit SHA of the PR (`pr.Head.SHA`)
- `line` must refer to a line within the diff, not an arbitrary file line
- Comments on lines outside the diff will return `422 Unprocessable Entity`

### 9c. Delete Comment

```
DELETE /repos/{owner}/{repo}/pulls/comments/{comment_id}
```

**Note**: The path is `/pulls/comments/{comment_id}`, NOT `/pulls/{pull_number}/comments/{comment_id}`.

#### Response

- Status: `204 No Content` (success)
- Status: `404 Not Found` (comment doesn't exist)

---

## 10. Mapping to CRoBot Platform Interface

### Interface Methods → GitHub API Calls

```
Platform.GetPRContext(ctx, PRRequest)
  ├─ GET /repos/{owner}/{repo}/pulls/{pr_number}           → metadata
  ├─ GET /repos/{owner}/{repo}/pulls/{pr_number}/files      → changed files (paginated)
  └─ GET /repos/{owner}/{repo}/pulls/{pr_number}            → raw diff (Accept: application/vnd.github.diff)
      (parse unified diff into DiffHunks)

Platform.GetFileContent(ctx, FileRequest)
  └─ GET /repos/{owner}/{repo}/contents/{path}?ref={commit} → raw content
      (Accept: application/vnd.github.raw+json)

Platform.ListBotComments(ctx, PRRequest)
  └─ GET /repos/{owner}/{repo}/pulls/{pr_number}/comments   → paginated comments
      (filter by user.type=="Bot" or fingerprint match)

Platform.CreateInlineComment(ctx, PRRequest, InlineComment)
  └─ POST /repos/{owner}/{repo}/pulls/{pr_number}/comments  → new comment
      (map side: "new"->"RIGHT", "old"->"LEFT")
      (commit_id = HeadCommit from prior GetPRContext call)

Platform.DeleteComment(ctx, PRRequest, commentID)
  └─ DELETE /repos/{owner}/{repo}/pulls/comments/{comment_id}
      (NOTE: no PR number in path)
```

### Config Structure (proposed)

```go
type GitHubConfig struct {
    Owner string // repository owner (maps to PRRequest.Workspace)
    Repo  string // repository name
    Token string // personal access token or app token
}
```

### Key Differences from Bitbucket Adapter

| Aspect             | Bitbucket                         | GitHub                              |
|--------------------|-----------------------------------|-------------------------------------|
| Auth               | Basic Auth (user:token)           | Bearer token (no username needed)   |
| Base URL           | `api.bitbucket.org`               | `api.github.com`                    |
| PR state           | `OPEN`, `MERGED`, `DECLINED`      | `open`, `closed` + `merged` bool    |
| File status        | `added`, `modified`, `removed`, `renamed` | Same + `copied`, `changed`, `unchanged` |
| Comment side       | `inline.to` / `inline.from`      | `side: RIGHT/LEFT`, `line`          |
| Delete comment URL | Includes PR number                | Does NOT include PR number          |
| Diff retrieval     | Separate diff endpoint            | Accept header on PR endpoint        |
| File content       | Raw bytes from src endpoint       | Base64 or raw via Accept header     |
| Pagination         | JSON `next` field in response     | `Link` header with rel="next"       |
| Comment ID type    | String                            | Integer (convert to string)         |
| Bot detection      | Match on user identity             | `user.type == "Bot"` field          |
| Rate limit         | Custom headers                    | `x-ratelimit-*` headers             |
| API versioning     | None (URL path versioning)        | `X-GitHub-Api-Version` header       |
| User-Agent         | Optional                          | **Required** (rejected without it)  |

### Go Struct Definitions (for JSON unmarshaling)

```go
// PR metadata response
type ghPullRequest struct {
    Number int       `json:"number"`
    Title  string    `json:"title"`
    Body   string    `json:"body"`
    State  string    `json:"state"`  // "open" or "closed"
    Merged bool      `json:"merged"`
    User   ghUser    `json:"user"`
    Head   ghRef     `json:"head"`
    Base   ghRef     `json:"base"`
}

type ghUser struct {
    Login string `json:"login"`
    ID    int    `json:"id"`
    Type  string `json:"type"` // "User", "Bot", "Organization"
}

type ghRef struct {
    Ref  string `json:"ref"`
    SHA  string `json:"sha"`
    Repo ghRepo `json:"repo"`
}

type ghRepo struct {
    FullName string `json:"full_name"`
}

// Changed files response (array of these)
type ghDiffEntry struct {
    SHA              string `json:"sha"`
    Filename         string `json:"filename"`
    Status           string `json:"status"`
    Additions        int    `json:"additions"`
    Deletions        int    `json:"deletions"`
    Changes          int    `json:"changes"`
    Patch            string `json:"patch"`
    PreviousFilename string `json:"previous_filename,omitempty"`
    ContentsURL      string `json:"contents_url"`
}

// Review comment response
type ghComment struct {
    ID                int    `json:"id"`
    Body              string `json:"body"`
    Path              string `json:"path"`
    Line              int    `json:"line"`
    Side              string `json:"side"` // "LEFT" or "RIGHT"
    CommitID          string `json:"commit_id"`
    OriginalCommitID  string `json:"original_commit_id"`
    OriginalLine      int    `json:"original_line"`
    DiffHunk          string `json:"diff_hunk"`
    User              ghUser `json:"user"`
    CreatedAt         string `json:"created_at"`
    UpdatedAt         string `json:"updated_at"`
    HTMLURL           string `json:"html_url"`
    InReplyToID       int    `json:"in_reply_to_id,omitempty"`
    SubjectType       string `json:"subject_type"`
    AuthorAssociation string `json:"author_association"`
}

// Create comment request body
type ghCreateComment struct {
    Body     string `json:"body"`
    CommitID string `json:"commit_id"`
    Path     string `json:"path"`
    Line     int    `json:"line"`
    Side     string `json:"side"` // "RIGHT" or "LEFT"
}

// File content response
type ghFileContent struct {
    Type     string `json:"type"`
    Encoding string `json:"encoding"`
    Size     int    `json:"size"`
    Name     string `json:"name"`
    Path     string `json:"path"`
    Content  string `json:"content"`
    SHA      string `json:"sha"`
}
```

### URL Parsing (for `ParsePRURL`)

GitHub PR URLs follow this pattern:
```
https://github.com/{owner}/{repo}/pull/{number}
```

Regex: `^https?://github\.com/([^/]+)/([^/]+)/pull/(\d+)$`

Maps to:
```go
PRRequest{
    Workspace: owner,  // GitHub "owner" maps to CRoBot "workspace"
    Repo:      repo,
    PRNumber:  number,
}
```

---

## Gotchas and Edge Cases

1. **Comment `commit_id` must be HEAD**: If you use a stale commit SHA, the comment may show as "outdated" or fail with 422.

2. **Delete path has no PR number**: `DELETE /repos/{owner}/{repo}/pulls/comments/{comment_id}` — this is different from Bitbucket.

3. **`line` must be in the diff**: Posting a comment on a line not in the diff returns `422 Unprocessable Entity`.

4. **3000 file limit**: PRs with >3000 changed files will silently truncate. Consider warning the user.

5. **Binary files have no `patch`**: The `patch` field is omitted for binary files in the files endpoint.

6. **Rate limit response codes**: Primary limits return `403` (not `429`). Secondary/abuse limits return `429` with `retry-after`. Check both.

7. **User-Agent required**: Unlike Bitbucket, GitHub rejects requests without a `User-Agent` header.

8. **Merged state**: GitHub doesn't have a separate `"merged"` state string. A PR is `state: "closed"` with `merged: true`.

9. **Pagination via headers (not body)**: Unlike Bitbucket's JSON `next` field, GitHub uses `Link` HTTP headers for pagination.

10. **Comment IDs are integers**: GitHub uses integer IDs (Bitbucket uses strings). Convert with `strconv.Itoa()` / `strconv.Atoi()`.
