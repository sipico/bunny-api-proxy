> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bunny.net/llms.txt
> Use this file to discover all available pages before exploring further.

# Get DNS Query Statistics

> Returns the statistics for the DNS Zone with the given ID



## OpenAPI

````yaml https://core-api-public-docs.b-cdn.net/docs/v3/public.json get /dnszone/{id}/statistics
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
  /dnszone/{id}/statistics:
    get:
      tags:
        - DNS Zone
      summary: Get DNS Query Statistics
      description: Returns the statistics for the DNS Zone with the given ID
      operationId: DnsZonePublic_Statistics
      parameters:
        - name: id
          in: path
          description: The ID of the DNS Zone for which the statistics will be returned
          schema:
            type: integer
            format: int64
          required: true
        - name: dateFrom
          in: query
          description: >-
            (Optional) The start date of the statistics. If no value is passed,
            the last 30 days will be returned
          x-nullable: true
          schema:
            type: string
            format: date-time
        - name: dateTo
          in: query
          description: >-
            (Optional) The end date of the statistics. If no value is passed,
            the last 30 days will be returned
          x-nullable: true
          schema:
            type: string
            format: date-time
      responses:
        '200':
          description: Returns the statistics for the DNS Zone with the given ID
          x-nullable: false
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DnsZoneStatisticsModel'
            application/xml:
              schema:
                $ref: '#/components/schemas/DnsZoneStatisticsModel'
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
    DnsZoneStatisticsModel:
      type: object
      additionalProperties: false
      properties:
        TotalQueriesServed:
          type: integer
          format: int64
        QueriesServedChart:
          type: object
          additionalProperties:
            type: number
            format: double
          nullable: true
        NormalQueriesServedChart:
          type: object
          additionalProperties:
            type: number
            format: double
          nullable: true
        SmartQueriesServedChart:
          type: object
          additionalProperties:
            type: number
            format: double
          nullable: true
        QueriesByTypeChart:
          type: object
          additionalProperties:
            type: integer
            format: int64
          nullable: true
      required:
        - TotalQueriesServed
  securitySchemes:
    AccessKey:
      type: apiKey
      description: API Access Key authorization header
      name: AccessKey
      in: header

````