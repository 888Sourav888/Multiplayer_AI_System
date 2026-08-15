package com.multiplayerai.service.dto;

import jakarta.validation.Valid;
import jakarta.validation.constraints.NotEmpty;
import jakarta.validation.constraints.NotNull;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.util.List;
import java.util.UUID;

@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
@com.fasterxml.jackson.annotation.JsonIgnoreProperties(ignoreUnknown = true)
public class PatchTransferRequest {

    private String type = "PATCH_TRANSFER";

    @NotNull(message = "Session ID must not be null")
    private UUID sessionId;

    @NotNull(message = "Sender ID must not be null")
    private UUID senderId;

    private long timestamp = System.currentTimeMillis();

    @NotEmpty(message = "Patches list must not be empty")
    @Valid
    private List<FilePatchItem> patches;
}
