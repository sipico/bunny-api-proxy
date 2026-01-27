> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bunny.net/llms.txt
> Use this file to discover all available pages before exploring further.

# Add DNS Zone



## OpenAPI

````yaml https://core-api-public-docs.b-cdn.net/docs/v3/public.json post /dnszone
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
  /dnszone:
    post:
      tags:
        - DNS Zone
      summary: Add DNS Zone
      operationId: DnsZonePublic_Add
      parameters: []
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/DnsZoneAddModel'
          application/xml:
            schema:
              $ref: '#/components/schemas/DnsZoneAddModel'
        required: true
      responses:
        '201':
          description: The DNS Zone was successfuly added
        '400':
          description: Failed adding the DNS Zone. Model validation failed
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
    DnsZoneAddModel:
      type: object
      additionalProperties: false
      properties:
        Domain:
          type: string
          nullable: true
          description: The domain that will be added.
        Records:
          type: array
          items:
            $ref: '#/components/schemas/AddDnsRecordModel'
          nullable: true
          description: Optional array of DNS records to add when creating the zone.
      required:
        - Domain
    AddDnsRecordModel:
      type: object
      additionalProperties: false
      properties:
        Type:
          allOf:
            - $ref: '#/components/schemas/DnsRecordTypes'
          nullable: true
        Ttl:
          type: integer
          format: int32
          nullable: true
        Value:
          type: string
          nullable: true
        Name:
          type: string
          nullable: true
        Weight:
          type: integer
          format: int32
          nullable: true
        Priority:
          type: integer
          format: int32
          nullable: true
        Flags:
          type: integer
          minimum: 0
          maximum: 255
          nullable: true
        Tag:
          type: string
          nullable: true
        Port:
          type: integer
          format: int32
          nullable: true
        PullZoneId:
          type: integer
          format: int64
          nullable: true
        ScriptId:
          type: integer
          format: int64
          nullable: true
        Accelerated:
          type: boolean
          nullable: true
        MonitorType:
          allOf:
            - $ref: '#/components/schemas/DnsMonitoringType'
          nullable: true
        GeolocationLatitude:
          type: number
          format: double
          nullable: true
        GeolocationLongitude:
          type: number
          format: double
          nullable: true
        LatencyZone:
          type: string
          nullable: true
        SmartRoutingType:
          allOf:
            - $ref: '#/components/schemas/DnsSmartRoutingType'
          nullable: true
        Disabled:
          type: boolean
          nullable: true
        EnviromentalVariables:
          type: array
          items:
            $ref: '#/components/schemas/DnsRecordEnviromentalVariableModel'
          nullable: true
        Comment:
          type: string
          nullable: true
        AutoSslIssuance:
          type: boolean
          nullable: true
    DnsRecordTypes:
      type: string
      description: >-
        0 = A<br/>1 = AAAA<br/>2 = CNAME<br/>3 = TXT<br/>4 = MX<br/>5 =
        SPF<br/>6 = Flatten<br/>7 = PullZone<br/>8 = SRV<br/>9 = CAA<br/>10 =
        PTR<br/>11 = Script<br/>12 = NS
      x-enumNames:
        - A
        - AAAA
        - CNAME
        - TXT
        - MX
        - SPF
        - Flatten
        - PullZone
        - SRV
        - CAA
        - PTR
        - Script
        - NS
      enum:
        - A
        - AAAA
        - CNAME
        - TXT
        - MX
        - SPF
        - Flatten
        - PullZone
        - SRV
        - CAA
        - PTR
        - Script
        - NS
      x-enum-varnames:
        - A
        - AAAA
        - CNAME
        - TXT
        - MX
        - SPF
        - Flatten
        - PullZone
        - SRV
        - CAA
        - PTR
        - Script
        - NS
      example: A
    DnsMonitoringType:
      type: string
      description: 0 = None<br/>1 = Ping<br/>2 = Http<br/>3 = Monitor
      x-enumNames:
        - None
        - Ping
        - Http
        - Monitor
      enum:
        - None
        - Ping
        - Http
        - Monitor
      x-enum-varnames:
        - None
        - Ping
        - Http
        - Monitor
      example: None
    DnsSmartRoutingType:
      type: string
      description: 0 = None<br/>1 = Latency<br/>2 = Geolocation
      x-enumNames:
        - None
        - Latency
        - Geolocation
      enum:
        - None
        - Latency
        - Geolocation
      x-enum-varnames:
        - None
        - Latency
        - Geolocation
      example: None
    DnsRecordEnviromentalVariableModel:
      type: object
      additionalProperties: false
      properties:
        Name:
          type: string
          nullable: true
        Value:
          type: string
          nullable: true
  securitySchemes:
    AccessKey:
      type: apiKey
      description: API Access Key authorization header
      name: AccessKey
      in: header

````