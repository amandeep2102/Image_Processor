# Image Processing Service

A multi-tier application for uploading, processing, and storing images with metadata.

## Architecture

- **Client Tier**: Web clients interact with Frontend App (handles requests, no UI).
- **Application Tier**: Backend Server processes images (resize, thumbnail, convert, filter).
- **Data Tier**: Postgres DB (image metadata); Disk Storage (uploaded/processed images).

```mermaid
graph LR
    subgraph "Client Tier"
        Clients[Web Clients]
        Frontend[Frontend App<br/>Handles Requests]
    end
    subgraph "Application Tier"
        Backend[Backend Server<br/>Image Processing:<br/>- Resize<br/>- Thumbnail<br/>- Convert<br/>- Filter]
    end
    subgraph "Data Tier"
        DB[Postgres DB<br/>Stores Image Metadata]
        Disk[Disk Storage<br/>Stores Uploaded and<br/>Processed Images]
    end
    
    Clients -->|HTTP Requests| Frontend
    Frontend -->|Upload Images| Disk
    Frontend -->|Upload Metadata| DB
    Frontend -->|API Calls| Backend
    Backend -->|Fetch Original Metadata| DB
    Backend -->|Upload Processed Metadata| DB
    Backend -->|Read/Write Images| Disk
    Backend -->|Processed Data| Frontend
    Frontend -->|Responses| Clients