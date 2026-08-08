package com.multiplayerai.service.service;

import com.multiplayerai.service.dto.CreateSessionRequest;
import com.multiplayerai.service.dto.SessionResponse;
import com.multiplayerai.service.dto.SnapshotResponse;
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

        // Ensure user exists, if not create default user entity
        UserEntity owner = userRepository.findById(ownerId).orElseGet(() -> {
            UserEntity newUser = new UserEntity(ownerId, "user_" + ownerId.toString().substring(0, 8), ownerId + "@example.com", null);
            return userRepository.save(newUser);
        });

        // Enforce max active sessions per owner
        long activeCount = sessionRepository.countByOwnerIdAndStatus(ownerId, SessionStatus.ACTIVE);
        if (activeCount >= maxSessionsPerOwner) {
            throw new SessionLimitExceededException("Owner " + ownerId + " has reached the maximum allowed active sessions limit (" + maxSessionsPerOwner + ").");
        }

        // Create session entity to get ID generated
        SessionEntity session = new SessionEntity();
        session.setName(request.getName());
        session.setOwnerId(owner.getId());
        session.setCurrentVersion(1L);
        session.setStatus(SessionStatus.ACTIVE);

        // Pre-save to get UUID
        session = sessionRepository.save(session);

        // Setup server-side directory for this session
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
     * /DeleteSession - Marks session as TERMINATED and cleans up local server storage.
     */
    @Transactional
    public SessionResponse deleteSession(UUID sessionId) {
        SessionEntity session = sessionRepository.findById(sessionId)
                .orElseThrow(() -> new ResourceNotFoundException("Session with ID " + sessionId + " not found."));

        // Update status to TERMINATED
        session.setStatus(SessionStatus.TERMINATED);
        session.setLastActiveAt(OffsetDateTime.now());
        session = sessionRepository.save(session);

        // Clean up server local storage
        fileStorageService.deleteSessionDirectory(sessionId);

        return SessionResponse.fromEntity(session);
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
}
