users
-------------------------
id
username
email
password_hash
created_at
last_seen_at

Don't store passwords directly.

For your first version, you don't even need sophisticated authentication.

You could have:

User
id = UUID
display_name

and add proper authentication later.



sessions
-------------------------
id
name
owner
project_storage_path
current_version
created_at
last_active_at
status




session_members
-------------------------
session_id
user_id
joined_at
last_seen_at
role


snapshots
----------------------------------
id
session_id
version
storage_location
created_at


AIContext
-------------------------
id                  UUID PK
session_id          UUID FK → Session
provider            VARCHAR
model               VARCHAR
system_prompt       TEXT
context_version     BIGINT
created_at          TIMESTAMP
updated_at          TIMESTAMP



AIMessage
-------------------------
id                  UUID PK
ai_context_id      UUID FK → AIContext
sequence_number    BIGINT
role                ENUM
content             TEXT
created_at          TIMESTAMP



AIToolCall
-------------------------
id                  UUID PK
ai_context_id      UUID FK
message_id         UUID FK → AIMessage
tool_name           VARCHAR
arguments           JSONB
result              JSONB / TEXT
status              ENUM
created_at          TIMESTAMP
completed_at        TIMESTAMP


AIPatch
-------------------------
id                  UUID PK
ai_context_id      UUID FK
message_id         UUID FK → AIMessage
created_by         UUID
status              ENUM
patch_content      TEXT
created_at          TIMESTAMP
resolved_at         TIMESTAMP

