package com.multiplayerai.service.controller;

import com.multiplayerai.service.dto.CreateSessionRequest;
import com.multiplayerai.service.dto.SessionResponse;
import com.multiplayerai.service.dto.SnapshotResponse;
import com.multiplayerai.service.service.SessionService;
import jakarta.validation.Valid;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.multipart.MultipartFile;

import java.util.List;
import java.util.UUID;

@RestController
@RequestMapping("/api/sessions")
public class SessionController {

    @Autowired
    private SessionService sessionService;

    /**
     * 1. Create Session (/CreateSession)
     * Creates session, sets up server-side storage path, adds owner entry in session_members.
     */
    @PostMapping
    public ResponseEntity<SessionResponse> createSession(@Valid @RequestBody CreateSessionRequest request) {
        SessionResponse response = sessionService.createSession(request);
        return ResponseEntity.status(HttpStatus.CREATED).body(response);
    }

    /**
     * 2. Persist Session (/PersistSession)
     * Accepts a snapshot file (blob/zip) in payload, stores file on server disk, logs metadata in snapshots table.
     */
    @PostMapping(value = "/{sessionId}/persist", consumes = MediaType.MULTIPART_FORM_DATA_VALUE)
    public ResponseEntity<SnapshotResponse> persistSession(
            @PathVariable UUID sessionId,
            @RequestParam("file") MultipartFile file) {
        SnapshotResponse response = sessionService.persistSession(sessionId, file);
        return ResponseEntity.status(HttpStatus.CREATED).body(response);
    }

    /**
     * 3. Delete Session (/DeleteSession)
     * Deletes physical server storage directory and marks session as TERMINATED.
     */
    @DeleteMapping("/{sessionId}")
    public ResponseEntity<SessionResponse> deleteSession(@PathVariable UUID sessionId) {
        SessionResponse response = sessionService.deleteSession(sessionId);
        return ResponseEntity.ok(response);
    }

    /**
     * Get Session details by ID
     */
    @GetMapping("/{sessionId}")
    public ResponseEntity<SessionResponse> getSession(@PathVariable UUID sessionId) {
        SessionResponse response = sessionService.getSession(sessionId);
        return ResponseEntity.ok(response);
    }

    /**
     * Get all snapshots metadata for a session
     */
    @GetMapping("/{sessionId}/snapshots")
    public ResponseEntity<List<SnapshotResponse>> getSessionSnapshots(@PathVariable UUID sessionId) {
        List<SnapshotResponse> response = sessionService.getSessionSnapshots(sessionId);
        return ResponseEntity.ok(response);
    }
}
