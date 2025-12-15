db = db.getSiblingDB('linkedin_automation');

// Create collections with validation
db.createCollection('profiles', {
  validator: {
    $jsonSchema: {
      bsonType: 'object',
      required: ['linkedin_id', 'name', 'url', 'discovered_at'],
      properties: {
        linkedin_id: {
          bsonType: 'string',
          description: 'LinkedIn profile ID - required'
        },
        name: {
          bsonType: 'string',
          description: 'Profile name - required'
        },
        url: {
          bsonType: 'string',
          description: 'LinkedIn profile URL - required'
        },
        title: {
          bsonType: 'string',
          description: 'Job title'
        },
        company: {
          bsonType: 'string',
          description: 'Company name'
        },
        discovered_at: {
          bsonType: 'date',
          description: 'Discovery timestamp - required'
        },
        updated_at: {
          bsonType: 'date',
          description: 'Last update timestamp'
        }
      }
    }
  }
});

db.createCollection('connection_requests', {
  validator: {
    $jsonSchema: {
      bsonType: 'object',
      required: ['profile_id', 'status', 'sent_at'],
      properties: {
        profile_id: {
          bsonType: 'string',
          description: 'Reference to profile - required'
        },
        status: {
          bsonType: 'string',
          enum: ['sent', 'accepted', 'rejected', 'withdrawn', 'pending', 'failed'],
          description: 'Request status - required'
        },
        sent_at: {
          bsonType: 'date',
          description: 'Send timestamp - required'
        }
      }
    }
  }
});

db.createCollection('messages');
db.createCollection('activity_log');
db.createCollection('session_state');
db.createCollection('rate_limits');

print('Collections created successfully');
