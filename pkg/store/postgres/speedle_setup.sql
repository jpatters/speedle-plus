-- Setup the table to contain services and their policies
CREATE TABLE speedle_services (
    name text PRIMARY KEY,
    type character varying(100) NOT NULL,
    policies jsonb NOT NULL DEFAULT '[]'::jsonb,
    role_policies jsonb NOT NULL DEFAULT '[]'::jsonb,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb
);

-- Create a function that will

CREATE OR REPLACE FUNCTION notify_event() RETURNS TRIGGER AS $$

    DECLARE 
        data json;
        notification json;
    
    BEGIN
    
        -- Convert the old or new row to JSON, based on the kind of action.
        -- Action = DELETE?             -> OLD row
        -- Action = INSERT or UPDATE?   -> NEW row
        IF (TG_OP = 'DELETE') THEN
            data = row_to_json(OLD);
        ELSE
            data = row_to_json(NEW);
        END IF;
        
        -- Contruct the notification as a JSON string.
        notification = json_build_object(
                          'table',TG_TABLE_NAME,
                          'action', TG_OP,
                          'data', data);
        
                        
        -- Execute pg_notify(channel, notification)
        PERFORM pg_notify('events',notification::text);
        
        -- Result is ignored since this is an AFTER trigger
        RETURN NULL; 
    END;
    
$$ LANGUAGE plpgsql;


-- Now add the trigger that calls that function on table change
CREATE TRIGGER speedle_services_notify_event
AFTER INSERT OR UPDATE OR DELETE ON speedle_services
    FOR EACH ROW EXECUTE PROCEDURE notify_event();