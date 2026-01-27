> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bunny.net/llms.txt
> Use this file to discover all available pages before exploring further.

# Get the latest DNS record scan result for a DNS Zone



## OpenAPI

````yaml https://core-api-public-docs.b-cdn.net/docs/v3/public.json get /dnszone/{zoneId}/records/scan
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
  /dnszone/{zoneId}/records/scan:
    get:
      tags:
        - DNS Zone
      summary: Get the latest DNS record scan result for a DNS Zone
      operationId: TriggerDnsZoneRecordScan_GetLatestScan
      parameters:
        - name: zoneId
          in: path
          description: The DNS Zone ID
          schema:
            type: integer
            format: int64
          required: true
      responses:
        '200':
          description: Latest DNS record scan job details
          x-nullable: false
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DnsZoneRecordScanJobResponse'
            application/xml:
              schema:
                $ref: '#/components/schemas/DnsZoneRecordScanJobResponse'
        '400':
          x-nullable: false
          description: Failed removing hostname
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
          description: No scan found or DNS Zone not found
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
    DnsZoneRecordScanJobResponse:
      type: object
      additionalProperties: false
      properties:
        JobId:
          type: string
          format: uuid
        ZoneId:
          type: integer
          format: int64
          nullable: true
        Domain:
          type: string
          nullable: true
        AccountId:
          type: string
          nullable: true
        Status:
          nullable: true
          oneOf:
            - $ref: '#/components/schemas/DnsZoneScanJobStatus'
        CreatedAt:
          type: string
          format: date-time
        CompletedAt:
          type: string
          format: date-time
          nullable: true
        Records:
          type: array
          items:
            $ref: '#/components/schemas/DnsZoneDiscoveredRecordModel'
          nullable: true
        Error:
          type: string
          nullable: true
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
    DnsZoneDiscoveredRecordModel:
      type: object
      additionalProperties: false
      properties:
        Name:
          type: string
          nullable: true
          description: Record name relative to the zone. '@' represents apex.
        Type:
          allOf:
            - $ref: '#/components/schemas/DnsZoneDiscoveredRecordType'
          nullable: true
        Ttl:
          type: integer
          format: int32
          nullable: true
        Value:
          type: string
          nullable: true
        Priority:
          type: integer
          format: int32
          nullable: true
        Weight:
          type: integer
          format: int32
          nullable: true
        Port:
          type: integer
          format: int32
          nullable: true
        IsProxied:
          type: boolean
      required:
        - IsProxied
    DnsZoneDiscoveredRecordType:
      type: string
      description: >-
        0 = A<br/>1 = AAAA<br/>2 = CNAME<br/>3 = TXT<br/>4 = MX<br/>8 =
        SRV<br/>9 = CAA<br/>10 = PTR<br/>12 = NS<br/>13 = Svcb<br/>14 =
        HTTPS<br/>15 = TLSA<br/>16 = SOA
      x-enumNames:
        - A
        - AAAA
        - CNAME
        - TXT
        - MX
        - SRV
        - CAA
        - PTR
        - NS
        - Svcb
        - HTTPS
        - TLSA
        - SOA
      enum:
        - A
        - AAAA
        - CNAME
        - TXT
        - MX
        - SRV
        - CAA
        - PTR
        - NS
        - Svcb
        - HTTPS
        - TLSA
        - SOA
      x-enum-varnames:
        - A
        - AAAA
        - CNAME
        - TXT
        - MX
        - SRV
        - CAA
        - PTR
        - NS
        - Svcb
        - HTTPS
        - TLSA
        - SOA
      example: A
  securitySchemes:
    AccessKey:
      type: apiKey
      description: API Access Key authorization header
      name: AccessKey
      in: header

````