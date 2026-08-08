package com.multiplayerai.service.exception;

public class SessionLimitExceededException extends RuntimeException {
    public SessionLimitExceededException(String message) {
        super(message);
    }
}
