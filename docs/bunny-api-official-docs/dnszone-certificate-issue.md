> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bunny.net/llms.txt
> Use this file to discover all available pages before exploring further.

# Issue new wildcard certificate

> A certificate has been issued successfully



## OpenAPI

````yaml https://core-api-public-docs.b-cdn.net/docs/v3/public.json post /dnszone/{zoneId}/certificate/issue
openapi: 3.0.0
info:
  title: bunny.net API
  description: >-
    <img src='https://bunny.net/v2/images/bunnynet-logo-dark.svg' style='width:
    200px;' alt='bunny.net Logo'>
                   Learn how to use the [bunny.net](https://bunny.net "bunny.net - The content delivery platform that truly hops.") API. Everything that can be done with the control panel can also be achieved with our API documented on this page. To learn how to use the storage API, have a look at our <a href='https://bunnycdnstorage.docs.apiary.io/#'>storage API documentation</a>
                   <h2>Third party API clients:</h2> 
                   <br/>
                   We currently do not maintain an official API library, but you can use one of the third party ones provided here:<br/><br/>
                   <a rel='nofollow' href='https://github.com/codewithmark/bunnycdn'>https://github.com/codewithmark/bunnycdn</a> (bunny.net PHP library, thanks to <a rel="nofollow" href='https://codewithmark.com'>Code With Mark</a>)
                   <br/><br/>
                   <i style='font-size: 11px;'><b>Note that third party clients are not maintained or developed by bunny.net so we unfortunately cannot offer support for them.</b></i>
  termsOfService: https://bunny.net/tos
  contact:
    name: bunny.net
    url: https://docs.bunny.net
    email: support@bunny.net
  version: 1.0.0
servers:
  - url: https://api.bunny.net
    description: bunny.net API Server
security: []
paths:
  /dnszone/{zoneId}/certificate/issue:
    post:
      tags:
        - DNS Zone
      summary: Issue new wildcard certificate
      description: A certificate has been issued successfully
      operationId: DnsZonePublic_IssueWildcardCertificate
      parameters:
        - name: zoneId
          in: path
          description: The DNS Zone ID requiring a new certificate.
          schema:
            type: integer
            format: int64
          required: true
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/IssueWildcardCertificateRequestModel'
          application/xml:
            schema:
              $ref: '#/components/schemas/IssueWildcardCertificateRequestModel'
        required: true
      responses:
        '200':
          description: A certificate has been issued successfully
        '400':
          description: Failed to issue a new certificate
          x-nullable: false
          content:
            application/json:
              schema:
                $ref: f2306f15-d639-43bc-ba70-80858292260c
            application/xml:
              schema:
                $ref: f2306f15-d639-43bc-ba70-80858292260c
        '401':
          description: The request authorization failed
        '404':
          description: The DNS Zone with the requested ID does not exist.
        '500':
          description: Internal Server Error
        '503':
          description: The service is currently unavailable
      security:
        - AccessKey:
            - SubuserAPIDns
            - SubuserAPIManage
            - SubuserDns
            - SubuserManage
            - User
            - UserApi
components:
  schemas:
    IssueWildcardCertificateRequestModel:
      type: object
      additionalProperties: false
      properties:
        Domain:
          type: string
          nullable: true
  securitySchemes:
    AccessKey:
      type: apiKey
      description: API Access Key authorization header
      name: AccessKey
      in: header

````