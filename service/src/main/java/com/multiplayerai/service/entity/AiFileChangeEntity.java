package com.multiplayerai.service.entity;

import jakarta.persistence.*;
import lombok.Getter;
import lombok.Setter;
import java.time.OffsetDateTime;
import java.util.UUID;

@Entity
@Table(name = "ai_file_changes")
@Getter
@Setter
public class AiFileChangeEntity {

    @Id
    @GeneratedValue(strategy = GenerationType.AUTO)
    private UUID id;

    @Column(name = "session_id", nullable = false)
    private UUID sessionId;

    @Column(name = "sender_id", nullable = false)
    private UUID senderId;

    @Column(name = "event_timestamp", nullable = false)
    private OffsetDateTime eventTimestamp;

    @Column(name = "file_path", nullable = false)
    private String filePath;

    @Column(name = "file_name", nullable = false)
    private String fileName;

    @Column(name = "file_extension")
    private String fileExtension;

    @Column(name = "operation", nullable = false)
    private String operation;

    @Column(name = "size_bytes")
    private Long sizeBytes;

    @Column(name = "modifier", nullable = false)
    private String modifier;

    @Column(name = "is_ai_edit", nullable = false)
    private boolean isAiEdit;

    @Column(name = "is_revert", nullable = false)
    private boolean isRevert;

    @Column(name = "is_whole_file", nullable = false)
    private boolean isWholeFile;

    @Column(name = "content_delta", columnDefinition = "jsonb")
    private String contentDelta;

    @Column(name = "created_at", insertable = false, updatable = false)
    private OffsetDateTime createdAt;

    @PrePersist
    protected void onCreate() {
        if (eventTimestamp == null) {
            eventTimestamp = OffsetDateTime.now();
        }
    }
}
