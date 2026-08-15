package com.multiplayerai.service.service;

import com.multiplayerai.service.dto.CreateSessionRequest;
import com.multiplayerai.service.dto.SessionResponse;
import com.multiplayerai.service.dto.SnapshotResponse;
import com.multiplayerai.service.dto.UpdateSessionRequest;
import com.multiplayerai.service.entity.*;
import com.multiplayerai.service.enums.MemberRole;
import com.multiplayerai.service.enums.SessionStatus;
import com.multiplayerai.service.exception.ResourceNotFoundException;
import com.multiplayerai.service.exception.SessionLimitExceededException;
import com.multiplayerai.service.repository.*;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.multipart.MultipartFile;

import java.time.OffsetDateTime;
import java.util.List;
import java.util.UUID;
import java.util.stream.Collectors;

@Service
public class SessionService {

    @Autowired
    private SessionRepository sessionRepository;

    @Autowired
    private SessionMemberRepository sessionMemberRepository;

    @Autowired
    private SnapshotRepository snapshotRepository;

    @Autowired
    private UserRepository userRepository;

    @Autowired
    private FileStorageService fileStorageService;

    @Value("${app.session.max-per-owner:10}")
    private int maxSessionsPerOwner;

    /**
     * /CreateSession - Creates a new session, sets up server-side folder, and adds metadata.
     */
    @Transactional
    public SessionResponse createSession(CreateSessionRequest request) {
        UUID ownerId = request.getOwnerId();

        // Fetch owner user, throwing ResourceNotFoundException if user does not exist
        UserEntity owner = userRepository.findById(ownerId)
                .orElseThrow(() -> new ResourceNotFoundException("User not found with ID: " + ownerId + ". Please create a user first before creating a session."));

        // Enforce max active sessions per owner
        long activeCount = sessionRepository.countByOwnerIdAndStatus(ownerId, SessionStatus.ACTIVE);
        if (activeCount >= maxSessionsPerOwner) {
            throw new SessionLimitExceededException("Owner " + ownerId + " has reached the maximum allowed active sessions limit (" + maxSessionsPerOwner + ").");
        }

        // Initialize session entity without manually setting ID to let @GeneratedValue work properly
        SessionEntity session = new SessionEntity();
        session.setName(request.getName());
        session.setOwnerId(owner.getId());
        session.setProjectStoragePath("PENDING");
        session.setCurrentVersion(1L);
        session.setStatus(SessionStatus.ACTIVE);
        session.setGitRepoUrl(request.getGitRepoUrl());
        session.setGitBranch(request.getGitBranch());
        session.setGitCommitSha(request.getGitCommitSha());

        // Initial persist to get generated UUID from JPA
        session = sessionRepository.save(session);

        // Create server-side directory using generated UUID
        String storagePath = fileStorageService.createSessionDirectory(session.getId());
        session.setProjectStoragePath(storagePath);
        session = sessionRepository.save(session);

        // Add owner entry to session_members table
        SessionMemberId memberId = new SessionMemberId(session.getId(), owner.getId());
        SessionMemberEntity member = new SessionMemberEntity(memberId, MemberRole.OWNER);
        sessionMemberRepository.save(member);

        return SessionResponse.fromEntity(session);
    }

    /**
     * /PersistSession - Stores snapshot blob/zip on server disk, creates snapshot metadata entry, and updates session.
     */
    @Transactional
    public SnapshotResponse persistSession(UUID sessionId, MultipartFile snapshotFile) {
        SessionEntity session = sessionRepository.findById(sessionId)
                .orElseThrow(() -> new ResourceNotFoundException("Session with ID " + sessionId + " not found."));

        if (session.getStatus() == SessionStatus.TERMINATED) {
            throw new IllegalStateException("Cannot persist snapshot for a terminated session.");
        }

        if (snapshotFile == null || snapshotFile.isEmpty()) {
            throw new IllegalArgumentException("Snapshot file is empty.");
        }
        if (snapshotFile.getSize() > 5 * 1024 * 1024) { // 5MB limit
            throw new IllegalArgumentException("Snapshot file size exceeds the 5MB limit.");
        }

        // Increment version
        long nextVersion = session.getCurrentVersion() + 1;

        // Store physical file on disk
        String storageLocation = fileStorageService.storeSnapshot(sessionId, nextVersion, snapshotFile);

        // Save metadata entry in snapshots table
        SnapshotEntity snapshot = new SnapshotEntity(sessionId, nextVersion, storageLocation);
        snapshot = snapshotRepository.save(snapshot);

        // Update session version and last_active_at
        session.setCurrentVersion(nextVersion);
        session.setLastActiveAt(OffsetDateTime.now());
        sessionRepository.save(session);

        return SnapshotResponse.fromEntity(snapshot);
    }

    /**
     * /UpdateSession - Partially updates a session's mutable fields (name, status).
     * Only non-null fields in the request are applied.
     */
    @Transactional
    public SessionResponse updateSession(UUID sessionId, UpdateSessionRequest request) {
        SessionEntity session = sessionRepository.findById(sessionId)
                .orElseThrow(() -> new ResourceNotFoundException("Session with ID " + sessionId + " not found."));

        if (session.getStatus() == SessionStatus.TERMINATED) {
            throw new IllegalStateException("Cannot update a terminated session.");
        }

        if (request.getName() != null && !request.getName().isBlank()) {
            session.setName(request.getName());
        }

        if (request.getStatus() != null) {
            if (request.getStatus() == SessionStatus.TERMINATED) {
                throw new IllegalStateException("Use the delete endpoint to terminate a session.");
            }
            session.setStatus(request.getStatus());
        }

        if (request.getGitRepoUrl() != null) {
            session.setGitRepoUrl(request.getGitRepoUrl());
        }
        if (request.getGitBranch() != null) {
            session.setGitBranch(request.getGitBranch());
        }
        if (request.getGitCommitSha() != null) {
            session.setGitCommitSha(request.getGitCommitSha());
        }

        session.setLastActiveAt(OffsetDateTime.now());
        session = sessionRepository.save(session);

        return SessionResponse.fromEntity(session);
    }

    /**
     * /DeleteSession - Fully deletes session from DB and cleans up local server storage.
     */
    @Transactional
    public void deleteSession(UUID sessionId) {
        SessionEntity session = sessionRepository.findById(sessionId)
                .orElseThrow(() -> new ResourceNotFoundException("Session with ID " + sessionId + " not found."));

        // Clean up server local storage
        fileStorageService.deleteSessionDirectory(sessionId);

        // Fully remove the session from the database
        sessionRepository.delete(session);
    }

    /**
     * Retrieves session details by ID.
     */
    @Transactional(readOnly = true)
    public SessionResponse getSession(UUID sessionId) {
        SessionEntity session = sessionRepository.findById(sessionId)
                .orElseThrow(() -> new ResourceNotFoundException("Session with ID " + sessionId + " not found."));
        return SessionResponse.fromEntity(session);
    }

    /**
     * Lists all snapshots for a session.
     */
    @Transactional(readOnly = true)
    public List<SnapshotResponse> getSessionSnapshots(UUID sessionId) {
        if (!sessionRepository.existsById(sessionId)) {
            throw new ResourceNotFoundException("Session with ID " + sessionId + " not found.");
        }
        return snapshotRepository.findBySessionIdOrderByVersionDesc(sessionId)
                .stream()
                .map(SnapshotResponse::fromEntity)
                .collect(Collectors.toList());
    }

    /**
     * Lists all sessions owned by a specific user.
     */
    @Transactional(readOnly = true)
    public List<SessionResponse> getSessionsByOwner(UUID ownerId) {
        if (!userRepository.existsById(ownerId)) {
            throw new ResourceNotFoundException("User not found with ID: " + ownerId);
        }
        return sessionRepository.findByOwnerId(ownerId)
                .stream()
                .map(SessionResponse::fromEntity)
                .collect(Collectors.toList());
    }

    /**
     * Joins a user to an existing session by adding them as a session member.
     */
    @Transactional
    public SessionResponse joinSession(UUID sessionId, UUID userId) {
        SessionEntity session = sessionRepository.findById(sessionId)
                .orElseThrow(() -> new ResourceNotFoundException("Session with ID " + sessionId + " not found."));

        if (!userRepository.existsById(userId)) {
            throw new ResourceNotFoundException("User not found with ID: " + userId);
        }

        if (session.getStatus() != SessionStatus.ACTIVE) {
            throw new IllegalStateException("Cannot join a session that is not ACTIVE. Current status: " + session.getStatus());
        }

        SessionMemberId memberId = new SessionMemberId(sessionId, userId);
        if (sessionMemberRepository.existsById(memberId)) {
            // User is already a member (could be owner or member)
            return SessionResponse.fromEntity(session);
        }

        // Limit member count to at most 5
        long memberCount = sessionMemberRepository.countByIdSessionId(sessionId);
        if (memberCount >= 5) {
            throw new SessionLimitExceededException("Session " + sessionId + " has reached the maximum allowed members limit (5).");
        }

        SessionMemberEntity member = new SessionMemberEntity(memberId, MemberRole.MEMBER);
        sessionMemberRepository.save(member);

        // Update session activity time
        session.setLastActiveAt(OffsetDateTime.now());
        sessionRepository.save(session);

        return SessionResponse.fromEntity(session);
    }
}
