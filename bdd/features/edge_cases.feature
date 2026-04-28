Feature: Edge cases and malformed requests

  Background:
    Given a clean storage environment

  Scenario: Request to a path with a missing bucket segment returns 404
    When I make a GET request to "/objects/"
    Then the response status should be 404

  Scenario: Request to a path with a missing key segment returns 404
    When I make a GET request to "/objects/mybucket/"
    Then the response status should be 404
