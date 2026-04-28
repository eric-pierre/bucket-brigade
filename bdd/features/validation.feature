Feature: Request validation

  Background:
    Given a clean storage environment

  Scenario: Bucket name exceeding 255 characters is rejected
    When I upload object "file.txt" to a bucket with a name of 256 characters with content "data"
    Then the response status should be 400
    And the problem type should be "https://bucket-brigade.dev/problems/invalid-request"
    And the problem detail should be "bucket must be at most 255 characters"

  Scenario: Object key exceeding 255 characters is rejected
    When I upload an object with a key of 256 characters to bucket "test" with content "data"
    Then the response status should be 400
    And the problem type should be "https://bucket-brigade.dev/problems/invalid-request"
    And the problem detail should be "objectId must be at most 255 characters"

  Scenario: Upload body exceeding the size limit is rejected
    When I upload object "big.bin" to bucket "test" with a body exceeding the size limit
    Then the response status should be 413
    And the problem type should be "https://bucket-brigade.dev/problems/request-too-large"
    And the problem detail should be "request body exceeds configured upload limit"
