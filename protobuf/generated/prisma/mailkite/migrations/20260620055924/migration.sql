-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "bounce";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "campaign";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "mailinglist";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "media";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "subscriber";

-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "template";

-- CreateEnum
CREATE TYPE "bounce"."bounce_type" AS ENUM ('HARD', 'SOFT', 'COMPLAINT');

-- CreateEnum
CREATE TYPE "campaign"."campaign_type" AS ENUM ('REGULAR', 'OPTIN');

-- CreateEnum
CREATE TYPE "campaign"."campaign_state" AS ENUM ('DRAFT', 'SCHEDULED', 'RUNNING', 'PAUSED', 'CANCELLED', 'FINISHED');

-- CreateEnum
CREATE TYPE "mailinglist"."list_type" AS ENUM ('PUBLIC', 'PRIVATE');

-- CreateEnum
CREATE TYPE "mailinglist"."optin_type" AS ENUM ('SINGLE', 'DOUBLE');

-- CreateEnum
CREATE TYPE "campaign"."content_type" AS ENUM ('RICHTEXT', 'HTML', 'MARKDOWN', 'PLAIN', 'VISUAL');

-- CreateEnum
CREATE TYPE "subscriber"."subscriber_state" AS ENUM ('ENABLED', 'BLOCKLISTED');

-- CreateEnum
CREATE TYPE "subscriber"."subscription_state" AS ENUM ('UNCONFIRMED', 'CONFIRMED', 'UNSUBSCRIBED');

-- CreateEnum
CREATE TYPE "template"."template_type" AS ENUM ('CAMPAIGN', 'CAMPAIGN_VISUAL', 'TRANSACTIONAL');

-- CreateTable
CREATE TABLE "bounce"."resource" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "subscriber" TEXT,
    "campaign" TEXT,
    "campaign_display_name" TEXT,
    "email" TEXT,
    "type" "bounce"."bounce_type",
    "source" TEXT,
    "meta" JSONB,
    "create_time" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "resource_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "campaign"."resource" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "uuid" TEXT,
    "display_name" TEXT NOT NULL,
    "subject" TEXT NOT NULL,
    "sender_email" TEXT,
    "type" "campaign"."campaign_type",
    "format" "campaign"."content_type" NOT NULL DEFAULT 'RICHTEXT',
    "body" TEXT,
    "alt_body" TEXT,
    "template" TEXT,
    "tags" TEXT[],
    "messenger" TEXT,
    "headers" JSONB,
    "schedule_time" TIMESTAMP(3),
    "state" "campaign"."campaign_state",
    "archive" BOOLEAN,
    "archive_slug" TEXT,
    "archive_template" TEXT,
    "archive_meta" JSONB,
    "create_time" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMP(3) NOT NULL,
    "start_time" TIMESTAMP(3),
    "stats_id" TEXT,

    CONSTRAINT "resource_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "campaign"."stats" (
    "id" TEXT NOT NULL,
    "recipient_count" BIGINT,
    "sent" BIGINT,
    "views" BIGINT,
    "clicks" BIGINT,
    "bounces" BIGINT,

    CONSTRAINT "stats_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "campaign"."lists" (
    "id" TEXT NOT NULL,
    "campaign_id" TEXT NOT NULL,
    "mailing_list_id" TEXT NOT NULL,

    CONSTRAINT "lists_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "mailinglist"."resource" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "uuid" TEXT,
    "display_name" TEXT NOT NULL,
    "description" TEXT,
    "type" "mailinglist"."list_type" NOT NULL DEFAULT 'PUBLIC',
    "optin" "mailinglist"."optin_type" NOT NULL DEFAULT 'SINGLE',
    "tags" TEXT[],
    "subscriber_count" BIGINT,
    "subscriber_statuses" JSONB,
    "create_time" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "resource_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "media"."resource" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "uuid" TEXT,
    "filename" TEXT,
    "mime_type" TEXT,
    "url" TEXT,
    "thumbnail_url" TEXT,
    "create_time" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "resource_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "subscriber"."resource" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "uuid" TEXT,
    "email" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "state" "subscriber"."subscriber_state",
    "attributes" JSONB,
    "create_time" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "resource_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "subscriber"."subscriptions" (
    "id" TEXT NOT NULL,
    "list" TEXT,
    "list_display_name" TEXT,
    "state" "subscriber"."subscription_state",
    "create_time" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "subscriber_id" TEXT NOT NULL,

    CONSTRAINT "subscriptions_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "template"."resource" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "display_name" TEXT NOT NULL,
    "type" "template"."template_type" NOT NULL DEFAULT 'CAMPAIGN',
    "subject" TEXT,
    "body" TEXT NOT NULL,
    "is_default" BOOLEAN,
    "create_time" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "update_time" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "resource_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE UNIQUE INDEX "resource_name_key" ON "bounce"."resource"("name");

-- CreateIndex
CREATE INDEX "resource_subscriber_idx" ON "bounce"."resource"("subscriber");

-- CreateIndex
CREATE INDEX "resource_campaign_idx" ON "bounce"."resource"("campaign");

-- CreateIndex
CREATE UNIQUE INDEX "resource_name_key" ON "campaign"."resource"("name");

-- CreateIndex
CREATE INDEX "resource_template_idx" ON "campaign"."resource"("template");

-- CreateIndex
CREATE INDEX "resource_archive_template_idx" ON "campaign"."resource"("archive_template");

-- CreateIndex
CREATE INDEX "resource_stats_id_idx" ON "campaign"."resource"("stats_id");

-- CreateIndex
CREATE INDEX "lists_mailing_list_id_idx" ON "campaign"."lists"("mailing_list_id");

-- CreateIndex
CREATE UNIQUE INDEX "lists_campaign_id_mailing_list_id_key" ON "campaign"."lists"("campaign_id", "mailing_list_id");

-- CreateIndex
CREATE UNIQUE INDEX "resource_name_key" ON "mailinglist"."resource"("name");

-- CreateIndex
CREATE UNIQUE INDEX "resource_name_key" ON "media"."resource"("name");

-- CreateIndex
CREATE UNIQUE INDEX "resource_name_key" ON "subscriber"."resource"("name");

-- CreateIndex
CREATE INDEX "subscriptions_list_idx" ON "subscriber"."subscriptions"("list");

-- CreateIndex
CREATE INDEX "subscriptions_subscriber_id_idx" ON "subscriber"."subscriptions"("subscriber_id");

-- CreateIndex
CREATE UNIQUE INDEX "resource_name_key" ON "template"."resource"("name");

-- AddForeignKey
ALTER TABLE "bounce"."resource" ADD CONSTRAINT "resource_subscriber_fkey" FOREIGN KEY ("subscriber") REFERENCES "subscriber"."resource"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "bounce"."resource" ADD CONSTRAINT "resource_campaign_fkey" FOREIGN KEY ("campaign") REFERENCES "campaign"."resource"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "campaign"."resource" ADD CONSTRAINT "resource_template_fkey" FOREIGN KEY ("template") REFERENCES "template"."resource"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "campaign"."resource" ADD CONSTRAINT "resource_archive_template_fkey" FOREIGN KEY ("archive_template") REFERENCES "template"."resource"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "campaign"."resource" ADD CONSTRAINT "resource_stats_id_fkey" FOREIGN KEY ("stats_id") REFERENCES "campaign"."stats"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "campaign"."lists" ADD CONSTRAINT "lists_campaign_id_fkey" FOREIGN KEY ("campaign_id") REFERENCES "campaign"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "campaign"."lists" ADD CONSTRAINT "lists_mailing_list_id_fkey" FOREIGN KEY ("mailing_list_id") REFERENCES "mailinglist"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "subscriber"."subscriptions" ADD CONSTRAINT "subscriptions_list_fkey" FOREIGN KEY ("list") REFERENCES "mailinglist"."resource"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "subscriber"."subscriptions" ADD CONSTRAINT "subscriptions_subscriber_id_fkey" FOREIGN KEY ("subscriber_id") REFERENCES "subscriber"."resource"("id") ON DELETE CASCADE ON UPDATE CASCADE;
