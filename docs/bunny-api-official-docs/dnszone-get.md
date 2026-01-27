> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bunny.net/llms.txt
> Use this file to discover all available pages before exploring further.

# Get DNS Zone

> The DNS Zone with the requested ID



## OpenAPI

````yaml https://core-api-public-docs.b-cdn.net/docs/v3/public.json get /dnszone/{id}
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
  /dnszone/{id}:
    get:
      tags:
        - DNS Zone
      summary: Get DNS Zone
      description: The DNS Zone with the requested ID
      operationId: DnsZonePublic_Index2
      parameters:
        - name: id
          in: path
          description: The ID of the DNS Zone that will be returned
          schema:
            type: integer
            format: int64
          required: true
      responses:
        '200':
          description: The DNS Zone with the requested ID
          x-nullable: false
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DnsZoneModel'
            application/xml:
              schema:
                $ref: '#/components/schemas/DnsZoneModel'
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
    DnsZoneModel:
      type: object
      additionalProperties: false
      properties:
        Id:
          type: integer
          format: int64
        Domain:
          type: string
          nullable: true
        Records:
          type: array
          items:
            $ref: '#/components/schemas/DnsRecordModel'
          nullable: true
        DateModified:
          type: string
          format: date-time
        DateCreated:
          type: string
          format: date-time
        NameserversDetected:
          type: boolean
        CustomNameserversEnabled:
          type: boolean
        Nameserver1:
          type: string
          nullable: true
        Nameserver2:
          type: string
          nullable: true
        SoaEmail:
          type: string
          nullable: true
        NameserversNextCheck:
          type: string
          format: date-time
        LoggingEnabled:
          type: boolean
        LoggingIPAnonymizationEnabled:
          type: boolean
          description: Determines if the TLS 1 is enabled on the DNS Zone
        LogAnonymizationType:
          allOf:
            - $ref: '#/components/schemas/LogAnonymizationType'
          nullable: true
          description: Sets the log anonymization type for this zone
        DnsSecEnabled:
          type: boolean
          description: Determines if DNSSEC is enabled for this DNS Zone
        CertificateKeyType:
          description: The private key type to use for automatic certificates
          allOf:
            - $ref: '#/components/schemas/PrivateKeyType'
      required:
        - Id
        - DateModified
        - DateCreated
        - NameserversDetected
        - CustomNameserversEnabled
        - NameserversNextCheck
        - LoggingEnabled
        - LoggingIPAnonymizationEnabled
        - DnsSecEnabled
    DnsRecordModel:
      type: object
      additionalProperties: false
      properties:
        Id:
          type: integer
          format: int64
        Type:
          $ref: '#/components/schemas/DnsRecordTypes'
        Ttl:
          type: integer
          format: int32
        Value:
          type: string
          nullable: true
        Name:
          type: string
          nullable: true
        Weight:
          type: integer
          format: int32
        Priority:
          type: integer
          format: int32
        Port:
          type: integer
          format: int32
        Flags:
          type: integer
          minimum: 0
          maximum: 255
        Tag:
          type: string
          nullable: true
        Accelerated:
          type: boolean
        AcceleratedPullZoneId:
          type: integer
          format: int64
        LinkName:
          type: string
          nullable: true
        IPGeoLocationInfo:
          nullable: true
          oneOf:
            - $ref: '#/components/schemas/GeoDnsLocationModel'
        GeolocationInfo:
          nullable: true
          oneOf:
            - $ref: '#/components/schemas/DnsRecordGeoLocationInfo'
        MonitorStatus:
          nullable: true
          oneOf:
            - $ref: '#/components/schemas/DnsMonitoringStatus'
        MonitorType:
          $ref: '#/components/schemas/DnsMonitoringType'
        GeolocationLatitude:
          type: number
          format: double
        GeolocationLongitude:
          type: number
          format: double
        EnviromentalVariables:
          type: array
          items:
            $ref: '#/components/schemas/DnsRecordEnviromentalVariableModel'
          nullable: true
        LatencyZone:
          type: string
          nullable: true
        SmartRoutingType:
          $ref: '#/components/schemas/DnsSmartRoutingType'
        Disabled:
          type: boolean
        Comment:
          type: string
          nullable: true
        AutoSslIssuance:
          type: boolean
      required:
        - Id
        - Ttl
        - Weight
        - Priority
        - Port
        - Accelerated
        - AcceleratedPullZoneId
        - GeolocationLatitude
        - GeolocationLongitude
        - Disabled
        - AutoSslIssuance
    LogAnonymizationType:
      type: string
      description: 0 = OneDigit<br/>1 = Drop
      x-enumNames:
        - OneDigit
        - Drop
      enum:
        - OneDigit
        - Drop
      x-enum-varnames:
        - OneDigit
        - Drop
      example: OneDigit
    PrivateKeyType:
      type: string
      description: 0 = Ecdsa<br/>1 = Rsa
      x-enumNames:
        - Ecdsa
        - Rsa
      enum:
        - Ecdsa
        - Rsa
      x-enum-varnames:
        - Ecdsa
        - Rsa
      example: Ecdsa
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
    GeoDnsLocationModel:
      type: object
      additionalProperties: false
      properties:
        CountryCode:
          type: string
          nullable: true
          description: The ISO country code of the location
        Country:
          type: string
          nullable: true
          description: The name of the country of the location
        ASN:
          type: integer
          format: int64
          description: The ASN of the IP organization
        OrganizationName:
          type: string
          nullable: true
          description: The mame of the organization that owns the IP
        City:
          type: string
          nullable: true
          description: The name of the city of the location
      required:
        - ASN
      description: Billing model contains data summary about the user's billing
    DnsRecordGeoLocationInfo:
      type: object
      additionalProperties: false
      properties:
        Country:
          type: string
          nullable: true
        City:
          type: string
          nullable: true
        Latitude:
          type: number
          format: double
        Longitude:
          type: number
          format: double
      required:
        - Latitude
        - Longitude
    DnsMonitoringStatus:
      type: string
      description: 0 = Unknown<br/>1 = Online<br/>2 = Offline
      x-enumNames:
        - Unknown
        - Online
        - Offline
      enum:
        - Unknown
        - Online
        - Offline
      x-enum-varnames:
        - Unknown
        - Online
        - Offline
      example: Unknown
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
  securitySchemes:
    AccessKey:
      type: apiKey
      description: API Access Key authorization header
      name: AccessKey
      in: header

````