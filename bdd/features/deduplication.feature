Feature: Content-addressed deduplication (bucket-scoped)

  Background:
    Given a clean storage environment

  Scenario: Same content uploaded twice to the same bucket is deduplicated
    When I upload object "copy1.txt" to bucket "test" with content "shared data"
    And I upload object "copy2.txt" to bucket "test" with content "shared data"
    Then the data folder should contain 1 content file
    When I download object "copy1.txt" from bucket "test"
    Then the response status should be 200
    And the response body should be "shared data"
    When I download object "copy2.txt" from bucket "test"
    Then the response status should be 200
    And the response body should be "shared data"

  Scenario: Same content in different buckets creates separate files
    When I upload object "a.txt" to bucket "bucket1" with content "shared data"
    And I upload object "b.txt" to bucket "bucket2" with content "shared data"
    Then the data folder should contain 2 content files
    When I download object "a.txt" from bucket "bucket1"
    Then the response status should be 200
    And the response body should be "shared data"
    When I download object "b.txt" from bucket "bucket2"
    Then the response status should be 200
    And the response body should be "shared data"

  Scenario: Deleting one copy in a bucket leaves the other intact
    Given object "copy1.txt" in bucket "test" contains "shared data"
    And object "copy2.txt" in bucket "test" contains "shared data"
    When I delete object "copy1.txt" from bucket "test"
    Then the response status should be 200
    When I download object "copy2.txt" from bucket "test"
    Then the response status should be 200
    And the response body should be "shared data"

  Scenario: Replacing a shared object does not affect the other copy
    Given object "original.txt" in bucket "test" contains "shared data"
    And object "other.txt" in bucket "test" contains "shared data"
    When I upload object "original.txt" to bucket "test" with content "new data"
    Then the response status should be 201
    When I download object "other.txt" from bucket "test"
    Then the response status should be 200
    And the response body should be "shared data"

  Scenario: Overwriting one deduplicated object updates refcounts and file count correctly
    Given object "x.txt" in bucket "test" contains "hello"
    And object "y.txt" in bucket "test" contains "hello"
    Then the data folder should contain 1 content file
    When I upload object "x.txt" to bucket "test" with content "goodbye"
    Then the response status should be 201
    And the data folder should contain 2 content files
    When I download object "x.txt" from bucket "test"
    Then the response status should be 200
    And the response body should be "goodbye"
    When I download object "y.txt" from bucket "test"
    Then the response status should be 200
    And the response body should be "hello"
