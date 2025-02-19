use diesel::{prelude::*, Connection as _};
use diesel_migrations::{embed_migrations, EmbeddedMigrations, MigrationHarness};
use dotenvy::dotenv;
use std::env;
use amiquip::{Connection, Result};

fn main() {
    pub const MIGRATIONS: EmbeddedMigrations = embed_migrations!("migrations");

    let mut connection = {
        dotenv().ok();

        let database_url = env::var("DATABASE_URL").expect("DATABASE_URL must be set");
        let conn = diesel::PgConnection::establish(&database_url)
            .unwrap_or_else(|_| panic!("Error connecting to {}", database_url));
        conn
    };
    println!("Connected to database");

    connection.run_pending_migrations(MIGRATIONS).unwrap();

    let rabbitmq_url = env::var("RABBITMQ_URL").expect("RABBITMQ_URL must be set");

    let rabbitmq_connection = amiquip::Connection::insecure_open(&rabbitmq_url)
        .unwrap_or_else(|_| panic!("Error connecting to RabbitMQ {}", rabbitmq_url));

    println!("Connected to queue");

    rabbitmq_connection.close().expect("Error closing RabbitMQ connection");
}