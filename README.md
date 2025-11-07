# Image Processing Service

A multi-tier application for uploading, processing, and storing images with metadata.

## Architecture

- **Client Tier**: Web clients interact with Frontend App (handles requests, no UI).
- **Application Tier**: Backend Server processes images (resize, thumbnail, convert, filter).
- **Data Tier**: Postgres DB (image metadata); Disk Storage (uploaded/processed images).

```mermaid
graph TD

%% =====================
%% STYLE DEFINITIONS
%% =====================
classDef client fill:#fff2cc,stroke:#333,stroke-width:1px,color:#000;
classDef app fill:#c9daf8,stroke:#333,stroke-width:1px,color:#000;
classDef data fill:#f4cccc,stroke:#333,stroke-width:1px,color:#000;

%% =====================
%% CLIENT TIER
%% =====================
subgraph Client_Tier["***Client Tier***"]
    Clients[**Web Clients**]:::client
    Frontend[**Frontend App**<br/>Handles Requests]:::app
end

%% =====================
%% APPLICATION TIER
%% =====================
subgraph Application_Tier["***Application Tier***"]
    Backend[**Backend Server**<br/>Image Processing:<br/>• Resize<br/>• Thumbnail<br/>• Convert<br/>• Filter]:::app
end

%% =====================
%% DATA TIER
%% =====================
subgraph Data_Tier["***Data Tier***"]
    DB[**Postgres DB**<br/>Stores Uploaded and Processed Images Metadata]:::data
    Disk[**Disk Storage**<br/>Stores Uploaded and<br/>Processed Images]:::data
end

%% =====================
%% CONNECTIONS
%% =====================
Clients -->|HTTP Requests| Frontend
Frontend <-->|Upload / Download Images| Disk
Frontend -->|Upload / Fetch Metadata| DB
Frontend -->|API Calls| Backend
Backend -->|Upload / Fetch Metadata| DB
Backend <-->|Read / Write Images| Disk
Backend -->|Processed Data| Frontend
Frontend -->|Responses| Clients
```
