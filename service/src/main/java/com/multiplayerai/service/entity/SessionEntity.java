package com.multiplayerai.service.entity;

import com.multiplayerai.service.enums.SessionStatus;
import jakarta.persistence.*;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.time.OffsetDateTime;
import java.util.UUID;

@Entity
@Table(name = "sessions")
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public class SessionEntity {

    @Id
    @GeneratedValue(strategy = GenerationType.AUTO)
    @Column(name = "id", nullable = false, updatable = false)
    private UUID id;

    @Column(name = "name", nullable = false, length = 255)
    private String name;

    @Column(name = "owner_id", nullable = false)
    private UUID ownerId;

    @Column(name = "project_storage_path", nullable = false, columnDefinition = "TEXT")
    private String projectStoragePath;

    @Column(name = "current_version", nullable = false)
    private Long currentVersion = 1L;

    @Enumerated(EnumType.STRING)
    @Column(name = "status", nullable = false)
    private SessionStatus status = SessionStatus.ACTIVE;

    @Column(name = "git_repo_url", length = 255)
    private String gitRepoUrl;

    @Column(name = "git_branch", length = 100)
    private String gitBranch;

    @Column(name = "git_commit_sha", length = 40)
    private String gitCommitSha;

    @Column(name = "created_at", nullable = false, updatable = false)
    private OffsetDateTime createdAt;

    @Column(name = "last_active_at", nullable = false)
    private OffsetDateTime lastActiveAt;

    @PrePersist
    protected void onCreate() {
        if (createdAt == null) {
            createdAt = OffsetDateTime.now();
        }
        if (lastActiveAt == null) {
            lastActiveAt = OffsetDateTime.now();
        }
        if (currentVersion == null) {
            currentVersion = 1L;
        }
        if (status == null) {
            status = SessionStatus.ACTIVE;
        }
    }
}
