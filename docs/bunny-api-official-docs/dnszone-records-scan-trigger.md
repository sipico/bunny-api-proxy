> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bunny.net/llms.txt
> Use this file to discover all available pages before exploring further.

# Trigger a background scan for pre-existing DNS records. Can use ZoneId for existing zones or Domain for pre-zone creation scenarios.



## OpenAPI

````yaml https://core-api-public-docs.b-cdn.net/docs/v3/public.json post /dnszone/records/scan
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
  /dnszone/records/scan:
    post:
      tags:
        - DNS Zone
      summary: >-
        Trigger a background scan for pre-existing DNS records. Can use ZoneId
        for existing zones or Domain for pre-zone creation scenarios.
      operationId: TriggerDnsZoneRecordScan_TriggerScan
      parameters: []
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/TriggerDnsZoneRecordScanRequest'
          application/xml:
            schema:
              $ref: '#/components/schemas/TriggerDnsZoneRecordScanRequest'
        required: true
      responses:
        '200':
          description: DNS record scan job triggered successfully
          x-nullable: false
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DnsZoneRecordScanTriggerResponse'
            application/xml:
              schema:
                $ref: '#/components/schemas/DnsZoneRecordScanTriggerResponse'
        '400':
          description: >-
            Invalid request - either ZoneId or Domain must be provided, but not
            both
        '401':
          description: The request authorization failed
        '404':
          description: The DNS Zone with the requested ID does not exist
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
    TriggerDnsZoneRecordScanRequest:
      type: object
      additionalProperties: false
      properties:
        ZoneId:
          type: integer
          format: int64
          nullable: true
          description: >-
            The ID of the DNS Zone to scan. Either ZoneId or Domain must be
            provided, but not both.
        Domain:
          type: string
          nullable: true
          description: >-
            The domain name to scan. Either ZoneId or Domain must be provided,
            but not both. Can be used even before creating the DNS zone.
    DnsZoneRecordScanTriggerResponse:
      type: object
      additionalProperties: false
      properties:
        JobId:
          type: string
          format: uuid
        Status:
          nullable: true
          oneOf:
            - $ref: '#/components/schemas/DnsZoneScanJobStatus'
      required:
        - JobId
    DnsZoneScanJobStatus:
      type: string
      enum:
        - Pending
        - InProgress
        - Completed
        - Failed
      x-enumNames:
        - Pending
        - InProgress
        - Completed
        - Failed
      description: 0 = Pending<br/>1 = InProgress<br/>2 = Completed<br/>3 = Failed
      example: Pending
  securitySchemes:
    AccessKey:
      type: apiKey
      description: API Access Key authorization header
      name: AccessKey
      in: header

````