package com.multiplayerai.service.dto;

import com.multiplayerai.service.enums.SessionStatus;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public class UpdateSessionRequest {

    private String name;

    private SessionStatus status;
}
