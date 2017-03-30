CREATE TABLE measurement (
        id serial PRIMARY KEY,
        name varchar(255),
        type varchar(255),
        value double precision,
        timestamp timestamp
        login varchar(255) REFERENCES login (id)
    );

CREATE TABLE login (
            id varchar(255) PRIMARY KEY,
            first_name varchar(255),
            last_name varchar(255),
            email varchar(255),
            key varchar(255)
        );


CREATE TABLE plot (
            id serial PRIMARY KEY,
            start_time timestamp,
            end_time timestamp,
            name varchar (255),
            login varchar(255) REFERENCES login (id)
        );

CREATE TABLE instrument (
    id serial PRIMARY KEY,
    name varchar(255),
    type varchar(255),
    plot int REFERENCES plot (id)
)
