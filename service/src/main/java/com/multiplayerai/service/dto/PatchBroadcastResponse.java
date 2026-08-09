package com.multiplayerai.service.dto;

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
public class PatchBroadcastResponse {

    private String type = "PATCH_BROADCAST";
    private UUID sessionId;
    private UUID senderId;
    private long timestamp;
    private List<FilePatchItem> patches;
}
