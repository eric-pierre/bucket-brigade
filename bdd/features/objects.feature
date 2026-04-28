Feature: Object storage CRUD

  Background:
    Given a clean storage environment

  Scenario: Upload a new object
    When I upload object "photo.jpg" to bucket "images" with content "fake image data"
    Then the response status should be 201
    And the response body should contain key "id" with value "photo.jpg"

  Scenario: Download an existing object
    Given object "notes.txt" in bucket "docs" contains "meeting notes"
    When I download object "notes.txt" from bucket "docs"
    Then the response status should be 200
    And the response body should be "meeting notes"
    And the Content-Length header should be "13"

  Scenario: Download a non-existent object returns 404
    When I download object "missing.txt" from bucket "docs"
    Then the response status should be 404
    And the problem type should be "https://bucket-brigade.dev/problems/object-not-found"
    And the problem detail should be "object not found"

  Scenario: Delete an object makes it unavailable
    Given object "temp.txt" in bucket "files" contains "temporary"
    When I delete object "temp.txt" from bucket "files"
    Then the response status should be 200
    When I download object "temp.txt" from bucket "files"
    Then the response status should be 404

  Scenario: Delete a non-existent object returns 404
    When I delete object "ghost.txt" from bucket "files"
    Then the response status should be 404
    And the problem type should be "https://bucket-brigade.dev/problems/object-not-found"
    And the problem detail should be "object not found"

  Scenario: Re-uploading an object replaces its content and removes the old blob
    Given object "doc.txt" in bucket "docs" contains "version one"
    When I upload object "doc.txt" to bucket "docs" with content "version two"
    Then the response status should be 201
    And the data folder should contain 1 content file
    When I download object "doc.txt" from bucket "docs"
    Then the response body should be "version two"

  Scenario: Re-uploading an object with identical content is idempotent
    Given object "doc.txt" in bucket "docs" contains "same content"
    When I upload object "doc.txt" to bucket "docs" with content "same content"
    Then the response status should be 201
    And the data folder should contain 1 content file
    When I download object "doc.txt" from bucket "docs"
    Then the response body should be "same content"

  Scenario: Same object key in different buckets are independent
    When I upload object "file.txt" to bucket "alpha" with content "alpha data"
    And I upload object "file.txt" to bucket "beta" with content "beta data"
    When I download object "file.txt" from bucket "alpha"
    Then the response status should be 200
    And the response body should be "alpha data"
    When I download object "file.txt" from bucket "beta"
    Then the response status should be 200
    And the response body should be "beta data"

  Scenario: Empty body is valid content
    When I upload object "empty.bin" to bucket "test" with an empty body
    Then the response status should be 201
    When I download object "empty.bin" from bucket "test"
    Then the response status should be 200
    And the response body should be empty
