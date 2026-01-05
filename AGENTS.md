# AGENTS.md

This document outlines the AI agents and automated systems that can be used to work with the SickRock project.

## Project Overview

SickRock is a no-code database web application builder that allows users to create web applications on top of real databases. It's built with:

- **Backend**: Go with gRPC/Connect protocol
- **Frontend**: Vue.js 3 with TypeScript
- **Database**: SQLite/MySQL support
- **Architecture**: Microservices with protobuf communication

## Available Agents

### 1. Database Schema Agent
**Purpose**: Automatically analyze and manage database schemas

**Capabilities**:
- Analyze existing database tables and relationships
- Generate table configurations automatically
- Suggest optimal column types and constraints
- Create foreign key relationships
- Generate migration scripts

**Use Cases**:
- Onboard new databases into SickRock
- Optimize existing table structures
- Generate documentation for database schemas
- Suggest performance improvements

**API Endpoints**:
- `GetDatabaseTables` - List all tables in a database
- `CreateTableConfiguration` - Create new table configurations
- `GetTableStructure` - Analyze table structure
- `AddTableColumn` - Add new columns to tables

### 2. UI Generation Agent
**Purpose**: Automatically generate user interfaces for database tables

**Capabilities**:
- Create table views with optimal column visibility
- Generate forms for data entry
- Create dashboard components
- Suggest UI layouts based on data types
- Generate responsive designs

**Use Cases**:
- Auto-generate CRUD interfaces for new tables
- Create custom views for specific use cases
- Generate dashboard layouts
- Optimize user experience based on data patterns

**API Endpoints**:
- `CreateTableView` - Create custom table views
- `UpdateTableView` - Modify existing views
- `GetTableViews` - List available views
- `GetDashboards` - Manage dashboard components

### 3. Data Analysis Agent
**Purpose**: Analyze data patterns and provide insights

**Capabilities**:
- Analyze data distribution and patterns
- Detect anomalies in data
- Generate reports and summaries
- Suggest data quality improvements
- Create data visualizations
- Process synthetic fields for enhanced data analysis

**Use Cases**:
- Data quality assessment
- Performance analysis
- Business intelligence reporting
- Data migration planning
- Time-based analysis using relative timestamps

**API Endpoints**:
- `GetSystemInfo` - Get system statistics
- `ListItems` - Query data with filters (includes synthetic `srCreatedRelative` and `srUpdatedRelative` fields)
- `GetMostRecentlyViewed` - Analyze usage patterns

**Synthetic Fields**:
The `ListItems` API automatically includes synthetic fields to enhance data analysis:
- `srCreatedRelative` - Relative time in seconds from the current timestamp when `srCreated` is set
- `srUpdatedRelative` - Relative time in seconds from the current timestamp when `srUpdated` is set
- These fields are included as top-level fields alongside `srCreated`, `srUpdated`, and `additionalFields`
- These fields enable easy time-based filtering and analysis without requiring client-side calculations
- When tables have an `sr_updated` column, it is automatically set to the current timestamp on updates

### 4. Authentication & Security Agent
**Purpose**: Manage user authentication and security

**Capabilities**:
- Generate secure API keys
- Manage user sessions
- Implement device code authentication
- Monitor security events
- Suggest security improvements

**Use Cases**:
- Automated user management
- Security audit and compliance
- API key rotation
- Device authentication flows

**API Endpoints**:
- `CreateAPIKey` - Generate API keys
- `GenerateDeviceCode` - Device authentication
- `ValidateToken` - Session management
- `ResetUserPassword` - Password management

### 5. Migration Agent
**Purpose**: Automatically handle database migrations and schema changes

**Capabilities**:
- Generate migration scripts
- Handle schema changes safely
- Backup data before migrations
- Rollback failed migrations
- Validate migration integrity

**Use Cases**:
- Automated schema updates
- Data migration between databases
- Version control for database changes
- Production deployment safety

**API Endpoints**:
- `ChangeColumnType` - Modify column types
- `DropColumn` - Remove columns safely
- `ChangeColumnName` - Rename columns
- `CreateForeignKey` - Add relationships

## Agent Integration Patterns

### 1. Webhook Integration
Agents can be integrated via webhooks to respond to events:
- Table creation events
- Data modification events
- User authentication events
- System health events

### 2. Scheduled Tasks
Agents can run on schedules for:
- Data analysis reports
- Security audits
- Performance monitoring
- Cleanup tasks

### 3. Real-time Processing
Agents can process data in real-time for:
- Live data validation
- Instant UI updates
- Real-time notifications
- Dynamic dashboard updates

## Development Guidelines

### Agent Development
When developing new agents:

1. **Use the existing gRPC API** - All agents should interact through the defined protobuf interface
2. **Follow the authentication pattern** - Use JWT tokens or API keys for authentication
3. **Implement proper error handling** - Use the standard error response format
4. **Log all activities** - Use the structured logging system
5. **Test with both SQLite and MySQL** - Ensure compatibility with both database engines
6. **Prefer existing UI libraries** - Use femtocrank (NPM library) and picocrank (Vue component library) for styling instead of creating new CSS rules

### Agent Deployment
Agents can be deployed as:
- **Standalone services** - Independent microservices
- **Embedded modules** - Integrated into the main application
- **External tools** - Command-line utilities or scripts

### Configuration
Agents should be configurable via:
- Environment variables
- Configuration files
- Database settings
- User preferences

### UI Styling Guidelines
When developing UI components for agents:

1. **Use femtocrank** - The project's NPM library provides consistent styling utilities and CSS classes
2. **Leverage picocrank** - The Vue component library offers pre-built components with proper styling
3. **Avoid custom CSS** - New CSS rules should generally be avoided in favor of existing library styles
4. **Maintain consistency** - Use the established design system to ensure UI consistency across the application
5. **Component composition** - Build complex UIs by composing existing picocrank components rather than creating new styled elements

**Example of preferred approach:**
```vue
<template>
  <!-- Use picocrank components -->
  <Section>
    <Button variant="primary" @click="handleAction">
      <!-- Use femtocrank styling classes -->
      <span class="text-sm font-medium">Action Button</span>
    </Button>
  </Section>
</template>
```

**Avoid:**
```vue
<template>
  <!-- Don't create custom styled elements -->
  <div class="custom-button custom-styling">
    <span style="color: blue; font-weight: bold;">Action Button</span>
  </div>
</template>

<style>
/* Avoid adding new CSS rules */
.custom-button {
  background: #007bff;
  /* ... */
}
</style>
```

## Example Agent Implementations

### Database Schema Analyzer
```go
// Example agent that analyzes database schemas
type SchemaAnalyzer struct {
    client *sickrockpbconnect.SickRockClient
}

func (sa *SchemaAnalyzer) AnalyzeDatabase(database string) (*SchemaAnalysis, error) {
    // Get all tables
    tables, err := sa.client.GetDatabaseTables(context.Background(),
        &sickrockpb.GetDatabaseTablesRequest{Database: database})

    // Analyze each table structure
    for _, table := range tables.Tables {
        structure, err := sa.client.GetTableStructure(context.Background(),
            &sickrockpb.GetTableStructureRequest{PageId: table.TableName})
        // ... analysis logic
    }

    return analysis, nil
}
```

### UI Generator
```typescript
// Example agent that generates UI components
class UIGenerator {
    private client: SickRockClient;

    async generateTableView(tableName: string): Promise<TableView> {
        const structure = await this.client.getTableStructure({
            pageId: tableName
        });

        const columns = structure.fields.map(field => ({
            columnName: field.name,
            isVisible: true,
            columnOrder: 0,
            sortOrder: ""
        }));

        return await this.client.createTableView({
            tableName,
            viewName: `${tableName}_default`,
            columns
        });
    }
}
```

## Monitoring and Observability

### Metrics
Agents should expose metrics for:
- Processing time
- Success/failure rates
- Resource usage
- Data processed

### Logging
Use structured logging with:
- Agent identification
- Operation context
- Performance metrics
- Error details

### Health Checks
Implement health checks for:
- Agent availability
- Database connectivity
- API responsiveness
- Resource utilization

## Security Considerations

### Authentication
- All agents must authenticate with valid tokens
- Use least privilege principle for permissions
- Implement proper session management

### Data Protection
- Encrypt sensitive data in transit and at rest
- Implement proper access controls
- Audit all data access

### API Security
- Validate all inputs
- Implement rate limiting
- Use HTTPS for all communications
- Regular security updates

## Contributing

When contributing agent implementations:

1. Follow the existing code patterns
2. Add comprehensive tests
3. Update this documentation
4. Include usage examples
5. Consider both SQLite and MySQL compatibility

## Future Enhancements

Potential future agent capabilities:
- **ML Data Insights Agent** - Machine learning-based data analysis
- **Performance Optimization Agent** - Automatic query optimization
- **Compliance Agent** - Automated compliance checking
- **Backup Agent** - Automated backup and recovery
- **Integration Agent** - Third-party system integration

## Support

For questions about agent development or integration:
- Discord: [SickRock Discord Server](https://discord.gg/jhYWWpNJ3v)
- GitHub Issues: [SickRock Issues](https://github.com/jamesread/SickRock/issues)
- Documentation: See the main README.md for project details

---

*This document is part of the SickRock project - a no-code database web application builder.*
