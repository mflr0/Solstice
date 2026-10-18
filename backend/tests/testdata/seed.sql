/*M!999999\- enable the sandbox mode */ 

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*M!100616 SET @OLD_NOTE_VERBOSITY=@@NOTE_VERBOSITY, NOTE_VERBOSITY=0 */;
DROP TABLE IF EXISTS `account_tokens`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `account_tokens` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `user_id` bigint(20) NOT NULL,
  `token_hash` char(64) NOT NULL,
  `purpose` varchar(10) NOT NULL,
  `created_at` datetime(3) NOT NULL,
  `expires_at` datetime(3) NOT NULL,
  `used_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_account_tokens_hash` (`token_hash`),
  KEY `ix_account_tokens_user` (`user_id`,`used_at`),
  KEY `ix_account_tokens_expires` (`expires_at`),
  CONSTRAINT `fk_account_tokens_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `account_tokens` DISABLE KEYS */;
/*!40000 ALTER TABLE `account_tokens` ENABLE KEYS */;
DROP TABLE IF EXISTS `api_tokens`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `api_tokens` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `user_id` bigint(20) NOT NULL,
  `name` varchar(255) NOT NULL,
  `token_hash` char(64) NOT NULL,
  `created_at` datetime(3) NOT NULL,
  `expires_at` datetime(3) DEFAULT NULL,
  `last_used_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_api_tokens_hash` (`token_hash`),
  KEY `ix_api_tokens_user` (`user_id`),
  CONSTRAINT `fk_api_tokens_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `api_tokens` DISABLE KEYS */;
/*!40000 ALTER TABLE `api_tokens` ENABLE KEYS */;
DROP TABLE IF EXISTS `attachments`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `attachments` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `issue_id` bigint(20) NOT NULL,
  `author_id` bigint(20) DEFAULT NULL,
  `filename` varchar(255) NOT NULL,
  `mime_type` varchar(255) NOT NULL,
  `size` bigint(20) NOT NULL,
  `created_at` datetime(3) NOT NULL,
  `file_missing` tinyint(1) NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `ix_attachments_issue` (`issue_id`),
  KEY `fk_attachments_author` (`author_id`),
  CONSTRAINT `fk_attachments_author` FOREIGN KEY (`author_id`) REFERENCES `users` (`id`) ON DELETE SET NULL,
  CONSTRAINT `fk_attachments_issue` FOREIGN KEY (`issue_id`) REFERENCES `issues` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=10100 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `attachments` DISABLE KEYS */;
INSERT INTO `attachments` VALUES (10000,10002,13,'logs-prod.txt','text/plain',1950,'2025-09-20 12:48:43.000',0);
INSERT INTO `attachments` VALUES (10001,10002,13,'capture.png','image/png',353,'2025-09-19 09:25:38.000',1);
INSERT INTO `attachments` VALUES (10002,10002,13,'spec.pdf','application/pdf',609,'2025-09-13 09:16:10.000',0);
INSERT INTO `attachments` VALUES (10003,10012,18,'export.csv','text/csv',906,'2025-10-10 15:22:12.000',0);
INSERT INTO `attachments` VALUES (10004,10012,18,'capture.png','image/png',351,'2025-10-06 10:26:30.000',0);
INSERT INTO `attachments` VALUES (10005,10019,17,'spec.pdf','application/pdf',609,'2025-10-17 12:25:25.000',0);
INSERT INTO `attachments` VALUES (10006,10019,17,'export.csv','text/csv',587,'2025-10-17 07:05:02.000',0);
INSERT INTO `attachments` VALUES (10007,10019,17,'maquette.jpg','image/jpeg',3050,'2025-10-09 16:44:45.000',0);
INSERT INTO `attachments` VALUES (10008,10035,3,'capture.png','image/png',327,'2025-11-15 08:00:03.000',0);
INSERT INTO `attachments` VALUES (10009,10035,3,'spec.pdf','application/pdf',609,'2025-11-04 09:35:43.000',0);
INSERT INTO `attachments` VALUES (10010,10036,18,'capture.png','image/png',315,'2025-11-05 15:31:31.000',0);
INSERT INTO `attachments` VALUES (10011,10036,18,'maquette.jpg','image/jpeg',3330,'2025-11-13 16:09:35.000',0);
INSERT INTO `attachments` VALUES (10012,10036,18,'export.csv','text/csv',744,'2025-11-21 17:09:14.000',0);
INSERT INTO `attachments` VALUES (10013,10043,19,'spec.pdf','application/pdf',609,'2025-11-17 08:41:59.000',0);
INSERT INTO `attachments` VALUES (10014,10044,10,'capture.png','image/png',319,'2025-12-01 17:17:47.000',0);
INSERT INTO `attachments` VALUES (10015,10044,10,'export.csv','text/csv',824,'2025-11-15 10:16:26.000',0);
INSERT INTO `attachments` VALUES (10016,10046,5,'capture.png','image/png',310,'2025-12-05 16:36:59.000',0);
INSERT INTO `attachments` VALUES (10017,10046,5,'spec.pdf','application/pdf',609,'2025-11-20 07:03:52.000',0);
INSERT INTO `attachments` VALUES (10018,10046,5,'export.csv','text/csv',906,'2025-11-20 07:38:28.000',0);
INSERT INTO `attachments` VALUES (10019,10051,2,'logs-prod.txt','text/plain',3185,'2025-12-03 13:37:46.000',0);
INSERT INTO `attachments` VALUES (10020,10052,11,'spec.pdf','application/pdf',609,'2025-12-03 17:40:47.000',0);
INSERT INTO `attachments` VALUES (10021,10052,11,'maquette.jpg','image/jpeg',2368,'2025-11-26 10:53:33.000',0);
INSERT INTO `attachments` VALUES (10022,10062,17,'maquette.jpg','image/jpeg',3427,'2025-12-08 17:00:18.000',0);
INSERT INTO `attachments` VALUES (10023,10062,17,'export.csv','text/csv',635,'2025-12-05 07:20:53.000',0);
INSERT INTO `attachments` VALUES (10024,10062,17,'spec.pdf','application/pdf',609,'2025-12-06 18:11:50.000',0);
INSERT INTO `attachments` VALUES (10025,10068,10,'capture.png','image/png',330,'2025-12-24 08:13:26.000',0);
INSERT INTO `attachments` VALUES (10026,10076,3,'capture.png','image/png',321,'2026-01-03 15:11:25.000',0);
INSERT INTO `attachments` VALUES (10027,10076,3,'logs-prod.txt','text/plain',2405,'2025-12-23 09:39:25.000',0);
INSERT INTO `attachments` VALUES (10028,10076,3,'spec.pdf','application/pdf',609,'2025-12-22 14:21:56.000',0);
INSERT INTO `attachments` VALUES (10029,10077,16,'spec.pdf','application/pdf',609,'2025-12-18 15:52:37.000',0);
INSERT INTO `attachments` VALUES (10030,10077,16,'capture.png','image/png',316,'2025-12-22 13:15:36.000',0);
INSERT INTO `attachments` VALUES (10031,10077,16,'export.csv','text/csv',852,'2026-01-02 08:54:54.000',0);
INSERT INTO `attachments` VALUES (10032,10079,8,'logs-prod.txt','text/plain',2726,'2025-12-19 14:16:29.000',0);
INSERT INTO `attachments` VALUES (10033,10079,8,'capture.png','image/png',307,'2026-01-02 08:53:59.000',0);
INSERT INTO `attachments` VALUES (10034,10079,8,'export.csv','text/csv',933,'2026-01-05 09:29:40.000',0);
INSERT INTO `attachments` VALUES (10035,10080,16,'capture.png','image/png',353,'2025-12-30 09:15:59.000',0);
INSERT INTO `attachments` VALUES (10036,10083,7,'export.csv','text/csv',581,'2026-01-15 16:18:05.000',0);
INSERT INTO `attachments` VALUES (10037,10084,19,'logs-prod.txt','text/plain',3047,'2026-01-03 12:06:51.000',0);
INSERT INTO `attachments` VALUES (10038,10084,19,'capture.png','image/png',349,'2025-12-31 15:01:46.000',0);
INSERT INTO `attachments` VALUES (10039,10085,9,'maquette.jpg','image/jpeg',3293,'2025-12-30 16:16:09.000',0);
INSERT INTO `attachments` VALUES (10040,10085,9,'capture.png','image/png',343,'2026-01-09 11:49:09.000',0);
INSERT INTO `attachments` VALUES (10041,10086,7,'capture.png','image/png',336,'2026-01-20 18:23:09.000',0);
INSERT INTO `attachments` VALUES (10042,10086,7,'export.csv','text/csv',746,'2026-01-04 12:28:57.000',0);
INSERT INTO `attachments` VALUES (10043,10086,7,'spec.pdf','application/pdf',609,'2026-01-12 15:08:22.000',0);
INSERT INTO `attachments` VALUES (10044,10089,10,'export.csv','text/csv',801,'2026-01-16 09:18:35.000',0);
INSERT INTO `attachments` VALUES (10045,10102,21,'logs-prod.txt','text/plain',2267,'2026-02-03 13:07:03.000',0);
INSERT INTO `attachments` VALUES (10046,10102,21,'capture.png','image/png',314,'2026-02-19 11:40:01.000',0);
INSERT INTO `attachments` VALUES (10047,10114,10,'logs-prod.txt','text/plain',2402,'2026-02-14 10:39:19.000',0);
INSERT INTO `attachments` VALUES (10048,10114,10,'export.csv','text/csv',911,'2026-03-01 14:15:39.000',0);
INSERT INTO `attachments` VALUES (10049,10114,10,'spec.pdf','application/pdf',609,'2026-02-22 08:00:33.000',0);
INSERT INTO `attachments` VALUES (10050,10133,6,'capture.png','image/png',310,'2026-03-17 07:41:22.000',0);
INSERT INTO `attachments` VALUES (10051,10133,6,'export.csv','text/csv',587,'2026-03-13 10:22:44.000',0);
INSERT INTO `attachments` VALUES (10052,10133,6,'maquette.jpg','image/jpeg',2794,'2026-03-17 18:44:54.000',0);
INSERT INTO `attachments` VALUES (10053,10147,10,'maquette.jpg','image/jpeg',3516,'2026-03-29 12:24:30.000',0);
INSERT INTO `attachments` VALUES (10054,10147,10,'spec.pdf','application/pdf',609,'2026-04-07 16:55:56.000',0);
INSERT INTO `attachments` VALUES (10055,10147,10,'export.csv','text/csv',695,'2026-04-10 09:44:21.000',0);
INSERT INTO `attachments` VALUES (10056,10152,10,'maquette.jpg','image/jpeg',3125,'2026-04-26 17:15:54.000',0);
INSERT INTO `attachments` VALUES (10057,10155,10,'logs-prod.txt','text/plain',3055,'2026-04-24 13:07:35.000',0);
INSERT INTO `attachments` VALUES (10058,10158,17,'logs-prod.txt','text/plain',3118,'2026-05-01 11:25:13.000',0);
INSERT INTO `attachments` VALUES (10059,10163,2,'logs-prod.txt','text/plain',3179,'2026-05-02 08:11:57.000',0);
INSERT INTO `attachments` VALUES (10060,10169,3,'maquette.jpg','image/jpeg',3456,'2026-05-07 06:42:50.000',0);
INSERT INTO `attachments` VALUES (10061,10169,3,'logs-prod.txt','text/plain',2015,'2026-05-04 16:27:39.000',0);
INSERT INTO `attachments` VALUES (10062,10171,14,'export.csv','text/csv',884,'2026-05-03 12:36:53.000',0);
INSERT INTO `attachments` VALUES (10063,10171,14,'maquette.jpg','image/jpeg',3147,'2026-05-02 10:03:40.000',0);
INSERT INTO `attachments` VALUES (10064,10171,14,'capture.png','image/png',320,'2026-05-12 08:37:13.000',0);
INSERT INTO `attachments` VALUES (10065,10180,14,'capture.png','image/png',313,'2026-05-24 14:28:45.000',0);
INSERT INTO `attachments` VALUES (10066,10180,14,'export.csv','text/csv',587,'2026-05-22 15:31:50.000',0);
INSERT INTO `attachments` VALUES (10067,10183,4,'capture.png','image/png',308,'2026-06-01 07:47:46.000',0);
INSERT INTO `attachments` VALUES (10068,10183,4,'spec.pdf','application/pdf',609,'2026-05-17 16:29:30.000',0);
INSERT INTO `attachments` VALUES (10069,10187,19,'spec.pdf','application/pdf',609,'2026-05-18 08:41:56.000',0);
INSERT INTO `attachments` VALUES (10070,10195,10,'maquette.jpg','image/jpeg',2873,'2026-05-24 11:48:22.000',0);
INSERT INTO `attachments` VALUES (10071,10195,10,'logs-prod.txt','text/plain',2657,'2026-06-08 11:01:09.000',0);
INSERT INTO `attachments` VALUES (10072,10195,10,'spec.pdf','application/pdf',609,'2026-06-10 14:41:43.000',0);
INSERT INTO `attachments` VALUES (10073,10220,11,'logs-prod.txt','text/plain',2787,'2026-07-05 16:47:57.000',0);
INSERT INTO `attachments` VALUES (10074,10220,11,'spec.pdf','application/pdf',609,'2026-07-03 12:55:23.000',0);
INSERT INTO `attachments` VALUES (10075,10220,11,'export.csv','text/csv',825,'2026-06-19 12:10:06.000',0);
INSERT INTO `attachments` VALUES (10076,10225,14,'logs-prod.txt','text/plain',2987,'2026-06-21 15:38:13.000',0);
INSERT INTO `attachments` VALUES (10077,10227,21,'spec.pdf','application/pdf',609,'2026-07-07 11:43:05.000',0);
INSERT INTO `attachments` VALUES (10078,10227,21,'capture.png','image/png',317,'2026-07-10 13:47:25.000',0);
INSERT INTO `attachments` VALUES (10079,10241,19,'logs-prod.txt','text/plain',3185,'2026-07-29 15:33:53.000',0);
INSERT INTO `attachments` VALUES (10080,10248,23,'capture.png','image/png',305,'2026-08-10 15:09:43.000',0);
INSERT INTO `attachments` VALUES (10081,10248,23,'export.csv','text/csv',587,'2026-08-12 14:41:26.000',0);
INSERT INTO `attachments` VALUES (10082,10257,10,'export.csv','text/csv',612,'2026-08-10 13:54:50.000',0);
INSERT INTO `attachments` VALUES (10083,10257,10,'logs-prod.txt','text/plain',2145,'2026-08-17 11:19:57.000',0);
INSERT INTO `attachments` VALUES (10084,10258,9,'logs-prod.txt','text/plain',2210,'2026-08-07 08:59:32.000',0);
INSERT INTO `attachments` VALUES (10085,10258,9,'spec.pdf','application/pdf',609,'2026-08-09 17:39:42.000',0);
INSERT INTO `attachments` VALUES (10086,10260,12,'export.csv','text/csv',717,'2026-08-17 13:15:53.000',0);
INSERT INTO `attachments` VALUES (10087,10260,12,'logs-prod.txt','text/plain',2405,'2026-08-21 15:36:02.000',0);
INSERT INTO `attachments` VALUES (10088,10260,12,'maquette.jpg','image/jpeg',2963,'2026-08-28 15:53:04.000',0);
INSERT INTO `attachments` VALUES (10089,10277,11,'export.csv','text/csv',798,'2026-09-18 07:07:36.000',0);
INSERT INTO `attachments` VALUES (10090,10280,17,'capture.png','image/png',348,'2026-09-13 13:35:10.000',0);
INSERT INTO `attachments` VALUES (10091,10280,17,'export.csv','text/csv',851,'2026-09-12 13:38:40.000',0);
INSERT INTO `attachments` VALUES (10092,10282,18,'logs-prod.txt','text/plain',2722,'2026-09-13 07:06:13.000',0);
INSERT INTO `attachments` VALUES (10093,10282,18,'spec.pdf','application/pdf',609,'2026-09-27 13:17:52.000',0);
INSERT INTO `attachments` VALUES (10094,10282,18,'maquette.jpg','image/jpeg',3464,'2026-09-15 08:33:09.000',0);
INSERT INTO `attachments` VALUES (10095,10284,15,'capture.png','image/png',321,'2026-09-22 06:30:31.000',0);
INSERT INTO `attachments` VALUES (10096,10284,15,'export.csv','text/csv',581,'2026-09-25 09:48:18.000',0);
INSERT INTO `attachments` VALUES (10097,10284,15,'maquette.jpg','image/jpeg',3134,'2026-09-24 08:25:26.000',0);
INSERT INTO `attachments` VALUES (10098,10295,6,'capture.png','image/png',323,'2026-10-01 16:00:00.000',0);
INSERT INTO `attachments` VALUES (10099,10295,6,'spec.pdf','application/pdf',609,'2026-09-25 10:23:14.000',0);
/*!40000 ALTER TABLE `attachments` ENABLE KEYS */;
DROP TABLE IF EXISTS `audit_log`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `audit_log` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) NOT NULL,
  `user_id` bigint(20) DEFAULT NULL,
  `username` varchar(255) DEFAULT NULL,
  `ip` varchar(64) DEFAULT NULL,
  `action` varchar(100) NOT NULL,
  `target_type` varchar(50) DEFAULT NULL,
  `target_id` varchar(100) DEFAULT NULL,
  `target_label` varchar(255) DEFAULT NULL,
  `details` mediumtext DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_audit_created` (`created_at`),
  KEY `ix_audit_user` (`user_id`,`created_at`),
  KEY `ix_audit_action` (`action`,`created_at`),
  CONSTRAINT `fk_audit_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `audit_log` DISABLE KEYS */;
/*!40000 ALTER TABLE `audit_log` ENABLE KEYS */;
DROP TABLE IF EXISTS `board_column_statuses`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `board_column_statuses` (
  `column_id` bigint(20) NOT NULL,
  `status_id` varchar(60) NOT NULL,
  `position` int(11) NOT NULL DEFAULT 0,
  PRIMARY KEY (`column_id`,`status_id`),
  KEY `fk_bcs_status` (`status_id`),
  CONSTRAINT `fk_bcs_column` FOREIGN KEY (`column_id`) REFERENCES `board_columns` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_bcs_status` FOREIGN KEY (`status_id`) REFERENCES `statuses` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `board_column_statuses` DISABLE KEYS */;
INSERT INTO `board_column_statuses` VALUES (1,'10000',0);
INSERT INTO `board_column_statuses` VALUES (2,'3',0);
INSERT INTO `board_column_statuses` VALUES (3,'10002',0);
INSERT INTO `board_column_statuses` VALUES (4,'10001',0);
INSERT INTO `board_column_statuses` VALUES (5,'10000',0);
INSERT INTO `board_column_statuses` VALUES (6,'3',0);
INSERT INTO `board_column_statuses` VALUES (7,'10002',0);
INSERT INTO `board_column_statuses` VALUES (8,'10001',0);
INSERT INTO `board_column_statuses` VALUES (9,'1',0);
INSERT INTO `board_column_statuses` VALUES (9,'4',1);
INSERT INTO `board_column_statuses` VALUES (10,'3',0);
INSERT INTO `board_column_statuses` VALUES (11,'5',0);
INSERT INTO `board_column_statuses` VALUES (11,'6',1);
INSERT INTO `board_column_statuses` VALUES (12,'1',0);
INSERT INTO `board_column_statuses` VALUES (12,'4',1);
INSERT INTO `board_column_statuses` VALUES (13,'3',0);
INSERT INTO `board_column_statuses` VALUES (14,'5',0);
INSERT INTO `board_column_statuses` VALUES (14,'6',1);
/*!40000 ALTER TABLE `board_column_statuses` ENABLE KEYS */;
DROP TABLE IF EXISTS `board_columns`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `board_columns` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `board_id` bigint(20) NOT NULL,
  `name` varchar(255) NOT NULL,
  `position` int(11) NOT NULL DEFAULT 0,
  `min_issues` int(11) DEFAULT NULL,
  `max_issues` int(11) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_bc_board` (`board_id`),
  CONSTRAINT `fk_bc_board` FOREIGN KEY (`board_id`) REFERENCES `boards` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=15 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `board_columns` DISABLE KEYS */;
INSERT INTO `board_columns` VALUES (1,1,'To do',0,NULL,NULL);
INSERT INTO `board_columns` VALUES (2,1,'In progress',1,NULL,NULL);
INSERT INTO `board_columns` VALUES (3,1,'En revue',2,NULL,NULL);
INSERT INTO `board_columns` VALUES (4,1,'Done',3,NULL,NULL);
INSERT INTO `board_columns` VALUES (5,2,'To do',0,NULL,NULL);
INSERT INTO `board_columns` VALUES (6,2,'In progress',1,NULL,5);
INSERT INTO `board_columns` VALUES (7,2,'En revue',2,1,NULL);
INSERT INTO `board_columns` VALUES (8,2,'Done',3,NULL,NULL);
INSERT INTO `board_columns` VALUES (9,3,'Ouvert',0,NULL,NULL);
INSERT INTO `board_columns` VALUES (10,3,'In progress',1,NULL,NULL);
INSERT INTO `board_columns` VALUES (11,3,'Résolu',2,NULL,NULL);
INSERT INTO `board_columns` VALUES (12,4,'Ouvert',0,NULL,NULL);
INSERT INTO `board_columns` VALUES (13,4,'In progress',1,NULL,NULL);
INSERT INTO `board_columns` VALUES (14,4,'Résolu',2,NULL,NULL);
/*!40000 ALTER TABLE `board_columns` ENABLE KEYS */;
DROP TABLE IF EXISTS `board_projects`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `board_projects` (
  `board_id` bigint(20) NOT NULL,
  `project_id` bigint(20) NOT NULL,
  PRIMARY KEY (`board_id`,`project_id`),
  KEY `ix_bp_project` (`project_id`),
  CONSTRAINT `fk_bp_board` FOREIGN KEY (`board_id`) REFERENCES `boards` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_bp_project` FOREIGN KEY (`project_id`) REFERENCES `projects` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `board_projects` DISABLE KEYS */;
INSERT INTO `board_projects` VALUES (1,10000);
INSERT INTO `board_projects` VALUES (2,10001);
INSERT INTO `board_projects` VALUES (3,10003);
INSERT INTO `board_projects` VALUES (4,10002);
/*!40000 ALTER TABLE `board_projects` ENABLE KEYS */;
DROP TABLE IF EXISTS `board_quick_filters`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `board_quick_filters` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `board_id` bigint(20) NOT NULL,
  `name` varchar(255) NOT NULL,
  `query` text NOT NULL,
  `description` varchar(255) DEFAULT NULL,
  `position` int(11) NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `ix_bqf_board` (`board_id`),
  CONSTRAINT `fk_bqf_board` FOREIGN KEY (`board_id`) REFERENCES `boards` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=13 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `board_quick_filters` DISABLE KEYS */;
INSERT INTO `board_quick_filters` VALUES (1,1,'Mes tickets','assignee = currentUser()','Ce qui m\'est assigné',0);
INSERT INTO `board_quick_filters` VALUES (2,1,'Bugs','issuetype = Bug',NULL,1);
INSERT INTO `board_quick_filters` VALUES (3,1,'Urgent et récent','priority in (Highest, High) AND updated >= -14d','Priorité haute, modifié ces deux dernières semaines',2);
INSERT INTO `board_quick_filters` VALUES (4,2,'Mes tickets','assignee = currentUser()','Ce qui m\'est assigné',0);
INSERT INTO `board_quick_filters` VALUES (5,2,'Bugs','issuetype = Bug',NULL,1);
INSERT INTO `board_quick_filters` VALUES (6,2,'Urgent et récent','priority in (Highest, High) AND updated >= -14d','Priorité haute, modifié ces deux dernières semaines',2);
INSERT INTO `board_quick_filters` VALUES (7,3,'Mes tickets','assignee = currentUser()','Ce qui m\'est assigné',0);
INSERT INTO `board_quick_filters` VALUES (8,3,'Bugs','issuetype = Bug',NULL,1);
INSERT INTO `board_quick_filters` VALUES (9,3,'Urgent et récent','priority in (Highest, High) AND updated >= -14d','Priorité haute, modifié ces deux dernières semaines',2);
INSERT INTO `board_quick_filters` VALUES (10,4,'Mes tickets','assignee = currentUser()','Ce qui m\'est assigné',0);
INSERT INTO `board_quick_filters` VALUES (11,4,'Bugs','issuetype = Bug',NULL,1);
INSERT INTO `board_quick_filters` VALUES (12,4,'Urgent et récent','priority in (Highest, High) AND updated >= -14d','Priorité haute, modifié ces deux dernières semaines',2);
/*!40000 ALTER TABLE `board_quick_filters` ENABLE KEYS */;
DROP TABLE IF EXISTS `board_swimlanes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `board_swimlanes` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `board_id` bigint(20) NOT NULL,
  `name` varchar(255) NOT NULL,
  `query` text DEFAULT NULL,
  `position` int(11) NOT NULL DEFAULT 0,
  `is_default` tinyint(1) NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `ix_bs_board` (`board_id`),
  CONSTRAINT `fk_bs_board` FOREIGN KEY (`board_id`) REFERENCES `boards` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `board_swimlanes` DISABLE KEYS */;
INSERT INTO `board_swimlanes` VALUES (1,2,'Bugs','issuetype = Bug',0,0);
INSERT INTO `board_swimlanes` VALUES (2,2,'Le reste',NULL,1,1);
/*!40000 ALTER TABLE `board_swimlanes` ENABLE KEYS */;
DROP TABLE IF EXISTS `boards`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `boards` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL,
  `kind` enum('kanban','scrum') NOT NULL,
  `filter_id` bigint(20) NOT NULL,
  `sub_query` text DEFAULT NULL,
  `owner_id` bigint(20) DEFAULT NULL,
  `swimlanes` varchar(20) NOT NULL DEFAULT 'none',
  `estimate` varchar(60) DEFAULT NULL,
  `done_cutoff` varchar(20) DEFAULT NULL,
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_boards_filter` (`filter_id`),
  KEY `fk_boards_owner` (`owner_id`),
  CONSTRAINT `fk_boards_filter` FOREIGN KEY (`filter_id`) REFERENCES `filters` (`id`),
  CONSTRAINT `fk_boards_owner` FOREIGN KEY (`owner_id`) REFERENCES `users` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `boards` DISABLE KEYS */;
INSERT INTO `boards` VALUES (1,'DEMO — Scrum','scrum',10004,NULL,2,'none','customfield_10100',NULL,'2026-10-01 18:00:00.000','2026-10-01 18:00:00.000');
INSERT INTO `boards` VALUES (2,'WEB — Kanban','kanban',10005,'fixVersion in unreleasedVersions() OR fixVersion is EMPTY',3,'custom',NULL,'2w','2026-10-01 18:00:00.000','2026-10-01 18:00:00.000');
INSERT INTO `boards` VALUES (3,'OPS — Exploitation','kanban',10006,NULL,7,'assignee',NULL,NULL,'2026-10-01 18:00:00.000','2026-10-01 18:00:00.000');
INSERT INTO `boards` VALUES (4,'RH','scrum',10007,NULL,8,'none','timeoriginalestimate',NULL,'2026-10-01 18:00:00.000','2026-10-01 18:00:00.000');
/*!40000 ALTER TABLE `boards` ENABLE KEYS */;
DROP TABLE IF EXISTS `change_groups`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `change_groups` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `issue_id` bigint(20) NOT NULL,
  `author_id` bigint(20) DEFAULT NULL,
  `created_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_cg_issue` (`issue_id`,`created_at`),
  KEY `fk_cg_author` (`author_id`),
  CONSTRAINT `fk_cg_author` FOREIGN KEY (`author_id`) REFERENCES `users` (`id`) ON DELETE SET NULL,
  CONSTRAINT `fk_cg_issue` FOREIGN KEY (`issue_id`) REFERENCES `issues` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=10858 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `change_groups` DISABLE KEYS */;
INSERT INTO `change_groups` VALUES (10000,10000,6,'2025-09-03 16:24:10.000');
INSERT INTO `change_groups` VALUES (10001,10000,15,'2025-09-25 11:58:36.000');
INSERT INTO `change_groups` VALUES (10002,10001,5,'2025-09-05 11:34:22.000');
INSERT INTO `change_groups` VALUES (10003,10001,5,'2025-09-11 07:23:32.000');
INSERT INTO `change_groups` VALUES (10004,10001,3,'2025-09-12 06:08:42.000');
INSERT INTO `change_groups` VALUES (10005,10002,2,'2025-09-03 11:25:58.000');
INSERT INTO `change_groups` VALUES (10006,10002,13,'2025-09-13 09:16:10.000');
INSERT INTO `change_groups` VALUES (10007,10002,2,'2025-09-18 13:50:47.000');
INSERT INTO `change_groups` VALUES (10008,10002,13,'2025-09-19 09:25:38.000');
INSERT INTO `change_groups` VALUES (10009,10002,13,'2025-09-20 12:48:43.000');
INSERT INTO `change_groups` VALUES (10010,10004,17,'2025-09-13 08:49:42.000');
INSERT INTO `change_groups` VALUES (10011,10004,11,'2025-12-14 18:40:36.000');
INSERT INTO `change_groups` VALUES (10012,10005,10,'2025-09-13 08:51:34.000');
INSERT INTO `change_groups` VALUES (10013,10005,5,'2025-09-18 08:43:33.000');
INSERT INTO `change_groups` VALUES (10014,10005,11,'2025-09-23 16:53:31.000');
INSERT INTO `change_groups` VALUES (10015,10005,11,'2025-10-03 09:47:10.000');
INSERT INTO `change_groups` VALUES (10016,10006,16,'2025-09-13 15:26:42.000');
INSERT INTO `change_groups` VALUES (10017,10007,6,'2025-09-27 16:25:11.000');
INSERT INTO `change_groups` VALUES (10018,10007,17,'2025-10-02 10:13:56.000');
INSERT INTO `change_groups` VALUES (10019,10007,6,'2025-12-15 13:54:17.000');
INSERT INTO `change_groups` VALUES (10020,10008,7,'2025-09-19 13:42:41.000');
INSERT INTO `change_groups` VALUES (10021,10008,5,'2025-09-28 14:27:35.000');
INSERT INTO `change_groups` VALUES (10022,10008,3,'2025-10-09 13:21:38.000');
INSERT INTO `change_groups` VALUES (10023,10009,6,'2025-10-02 17:17:29.000');
INSERT INTO `change_groups` VALUES (10024,10010,6,'2025-09-23 15:49:59.000');
INSERT INTO `change_groups` VALUES (10025,10010,15,'2025-10-04 14:47:24.000');
INSERT INTO `change_groups` VALUES (10026,10010,16,'2025-10-13 07:34:04.000');
INSERT INTO `change_groups` VALUES (10027,10011,4,'2025-09-24 17:33:05.000');
INSERT INTO `change_groups` VALUES (10028,10011,3,'2025-09-25 17:02:22.000');
INSERT INTO `change_groups` VALUES (10029,10011,3,'2025-10-01 11:06:48.000');
INSERT INTO `change_groups` VALUES (10030,10011,2,'2025-10-04 11:26:57.000');
INSERT INTO `change_groups` VALUES (10031,10011,2,'2026-03-19 14:43:31.000');
INSERT INTO `change_groups` VALUES (10032,10012,18,'2025-10-06 10:26:30.000');
INSERT INTO `change_groups` VALUES (10033,10012,18,'2025-10-10 15:22:12.000');
INSERT INTO `change_groups` VALUES (10034,10013,10,'2025-10-02 08:51:06.000');
INSERT INTO `change_groups` VALUES (10035,10014,14,'2025-10-05 15:03:38.000');
INSERT INTO `change_groups` VALUES (10036,10014,14,'2025-10-13 12:45:34.000');
INSERT INTO `change_groups` VALUES (10037,10015,11,'2025-10-08 06:16:29.000');
INSERT INTO `change_groups` VALUES (10038,10016,15,'2025-09-30 11:56:30.000');
INSERT INTO `change_groups` VALUES (10039,10016,7,'2025-09-30 17:07:09.000');
INSERT INTO `change_groups` VALUES (10040,10016,13,'2025-10-04 09:24:00.000');
INSERT INTO `change_groups` VALUES (10041,10018,3,'2025-10-10 11:03:45.000');
INSERT INTO `change_groups` VALUES (10042,10018,6,'2025-10-22 10:41:32.000');
INSERT INTO `change_groups` VALUES (10043,10018,7,'2025-10-26 10:10:40.000');
INSERT INTO `change_groups` VALUES (10044,10019,13,'2025-10-08 10:02:12.000');
INSERT INTO `change_groups` VALUES (10045,10019,17,'2025-10-09 16:44:45.000');
INSERT INTO `change_groups` VALUES (10046,10019,17,'2025-10-10 10:38:49.000');
INSERT INTO `change_groups` VALUES (10047,10019,17,'2025-10-17 07:05:02.000');
INSERT INTO `change_groups` VALUES (10048,10019,17,'2025-10-17 12:25:25.000');
INSERT INTO `change_groups` VALUES (10049,10019,17,'2026-04-29 15:05:41.000');
INSERT INTO `change_groups` VALUES (10050,10020,7,'2025-10-19 10:45:05.000');
INSERT INTO `change_groups` VALUES (10051,10020,7,'2025-10-23 09:59:07.000');
INSERT INTO `change_groups` VALUES (10052,10020,7,'2025-10-23 14:50:05.000');
INSERT INTO `change_groups` VALUES (10053,10021,12,'2025-10-16 15:02:09.000');
INSERT INTO `change_groups` VALUES (10054,10021,21,'2025-10-26 09:43:21.000');
INSERT INTO `change_groups` VALUES (10055,10021,15,'2025-10-27 14:48:16.000');
INSERT INTO `change_groups` VALUES (10056,10022,13,'2025-10-17 12:54:21.000');
INSERT INTO `change_groups` VALUES (10057,10022,15,'2025-10-26 14:55:30.000');
INSERT INTO `change_groups` VALUES (10058,10022,17,'2025-11-06 10:59:57.000');
INSERT INTO `change_groups` VALUES (10059,10022,16,'2025-11-06 17:04:32.000');
INSERT INTO `change_groups` VALUES (10060,10023,6,'2025-10-24 12:05:57.000');
INSERT INTO `change_groups` VALUES (10061,10023,10,'2025-10-24 17:08:17.000');
INSERT INTO `change_groups` VALUES (10062,10023,13,'2025-10-25 14:24:53.000');
INSERT INTO `change_groups` VALUES (10063,10024,15,'2025-10-25 10:14:38.000');
INSERT INTO `change_groups` VALUES (10064,10024,23,'2025-10-28 08:00:28.000');
INSERT INTO `change_groups` VALUES (10065,10024,21,'2025-11-07 13:34:35.000');
INSERT INTO `change_groups` VALUES (10066,10024,7,'2025-11-10 18:06:43.000');
INSERT INTO `change_groups` VALUES (10067,10024,22,'2025-11-13 08:36:56.000');
INSERT INTO `change_groups` VALUES (10068,10025,5,'2025-10-18 17:56:49.000');
INSERT INTO `change_groups` VALUES (10069,10025,17,'2025-10-22 13:32:25.000');
INSERT INTO `change_groups` VALUES (10070,10025,1,'2025-10-25 06:04:33.000');
INSERT INTO `change_groups` VALUES (10071,10025,17,'2025-11-17 17:41:37.000');
INSERT INTO `change_groups` VALUES (10072,10026,6,'2025-10-22 12:48:25.000');
INSERT INTO `change_groups` VALUES (10073,10026,15,'2025-10-25 15:05:51.000');
INSERT INTO `change_groups` VALUES (10074,10026,15,'2025-11-03 07:59:32.000');
INSERT INTO `change_groups` VALUES (10075,10026,16,'2025-11-07 15:04:27.000');
INSERT INTO `change_groups` VALUES (10076,10027,9,'2025-10-22 06:08:46.000');
INSERT INTO `change_groups` VALUES (10077,10027,18,'2025-10-24 16:18:25.000');
INSERT INTO `change_groups` VALUES (10078,10027,11,'2025-10-29 10:09:12.000');
INSERT INTO `change_groups` VALUES (10079,10027,7,'2025-11-10 13:13:03.000');
INSERT INTO `change_groups` VALUES (10080,10027,4,'2025-11-19 11:04:09.000');
INSERT INTO `change_groups` VALUES (10081,10028,10,'2025-10-29 15:31:09.000');
INSERT INTO `change_groups` VALUES (10082,10028,10,'2025-10-30 10:36:41.000');
INSERT INTO `change_groups` VALUES (10083,10029,11,'2025-10-22 12:32:00.000');
INSERT INTO `change_groups` VALUES (10084,10029,16,'2025-10-25 08:04:36.000');
INSERT INTO `change_groups` VALUES (10085,10029,11,'2025-10-28 14:32:13.000');
INSERT INTO `change_groups` VALUES (10086,10029,3,'2025-10-28 16:21:19.000');
INSERT INTO `change_groups` VALUES (10087,10029,11,'2025-10-30 12:33:21.000');
INSERT INTO `change_groups` VALUES (10088,10029,11,'2026-03-20 11:22:20.000');
INSERT INTO `change_groups` VALUES (10089,10030,11,'2025-10-22 17:29:53.000');
INSERT INTO `change_groups` VALUES (10090,10031,3,'2025-10-31 16:45:16.000');
INSERT INTO `change_groups` VALUES (10091,10033,11,'2025-11-02 16:55:06.000');
INSERT INTO `change_groups` VALUES (10092,10033,11,'2025-11-14 14:57:53.000');
INSERT INTO `change_groups` VALUES (10093,10034,16,'2025-11-07 11:33:26.000');
INSERT INTO `change_groups` VALUES (10094,10034,2,'2025-11-17 07:09:16.000');
INSERT INTO `change_groups` VALUES (10095,10034,16,'2026-01-29 09:25:41.000');
INSERT INTO `change_groups` VALUES (10096,10035,3,'2025-11-04 09:35:43.000');
INSERT INTO `change_groups` VALUES (10097,10035,4,'2025-11-13 12:29:25.000');
INSERT INTO `change_groups` VALUES (10098,10035,3,'2025-11-15 08:00:03.000');
INSERT INTO `change_groups` VALUES (10099,10035,5,'2025-11-19 07:05:18.000');
INSERT INTO `change_groups` VALUES (10100,10035,5,'2025-11-28 10:07:24.000');
INSERT INTO `change_groups` VALUES (10101,10036,18,'2025-11-05 15:31:31.000');
INSERT INTO `change_groups` VALUES (10102,10036,3,'2025-11-13 09:31:26.000');
INSERT INTO `change_groups` VALUES (10103,10036,18,'2025-11-13 16:09:35.000');
INSERT INTO `change_groups` VALUES (10104,10036,15,'2025-11-16 15:02:05.000');
INSERT INTO `change_groups` VALUES (10105,10036,18,'2025-11-21 17:09:14.000');
INSERT INTO `change_groups` VALUES (10106,10037,9,'2025-12-10 08:34:45.000');
INSERT INTO `change_groups` VALUES (10107,10038,4,'2025-11-09 18:08:47.000');
INSERT INTO `change_groups` VALUES (10108,10038,3,'2025-11-10 07:07:21.000');
INSERT INTO `change_groups` VALUES (10109,10039,7,'2025-11-06 18:56:49.000');
INSERT INTO `change_groups` VALUES (10110,10039,4,'2025-11-09 07:15:00.000');
INSERT INTO `change_groups` VALUES (10111,10039,4,'2025-11-09 13:12:07.000');
INSERT INTO `change_groups` VALUES (10112,10039,9,'2025-11-28 14:35:00.000');
INSERT INTO `change_groups` VALUES (10113,10040,6,'2025-11-13 10:03:08.000');
INSERT INTO `change_groups` VALUES (10114,10040,6,'2025-11-14 07:52:14.000');
INSERT INTO `change_groups` VALUES (10115,10040,9,'2025-11-18 18:04:16.000');
INSERT INTO `change_groups` VALUES (10116,10040,11,'2026-02-27 07:23:56.000');
INSERT INTO `change_groups` VALUES (10117,10041,4,'2025-11-10 14:10:37.000');
INSERT INTO `change_groups` VALUES (10118,10041,7,'2025-11-19 09:30:17.000');
INSERT INTO `change_groups` VALUES (10119,10041,4,'2025-11-19 15:56:30.000');
INSERT INTO `change_groups` VALUES (10120,10041,13,'2025-11-22 10:01:17.000');
INSERT INTO `change_groups` VALUES (10121,10041,4,'2025-12-01 10:49:01.000');
INSERT INTO `change_groups` VALUES (10122,10042,16,'2025-11-15 17:48:35.000');
INSERT INTO `change_groups` VALUES (10123,10042,4,'2025-11-22 10:16:59.000');
INSERT INTO `change_groups` VALUES (10124,10042,4,'2025-11-22 18:31:55.000');
INSERT INTO `change_groups` VALUES (10125,10042,4,'2025-12-03 11:05:56.000');
INSERT INTO `change_groups` VALUES (10126,10043,19,'2025-11-15 15:02:27.000');
INSERT INTO `change_groups` VALUES (10127,10043,19,'2025-11-17 08:41:59.000');
INSERT INTO `change_groups` VALUES (10128,10043,22,'2025-11-24 17:48:14.000');
INSERT INTO `change_groups` VALUES (10129,10044,10,'2025-11-15 10:16:26.000');
INSERT INTO `change_groups` VALUES (10130,10044,4,'2025-11-23 15:40:17.000');
INSERT INTO `change_groups` VALUES (10131,10044,10,'2025-12-01 17:17:47.000');
INSERT INTO `change_groups` VALUES (10132,10044,4,'2025-12-05 11:14:08.000');
INSERT INTO `change_groups` VALUES (10133,10044,4,'2025-12-11 15:48:07.000');
INSERT INTO `change_groups` VALUES (10134,10045,9,'2025-11-19 18:31:11.000');
INSERT INTO `change_groups` VALUES (10135,10045,9,'2025-11-23 08:13:24.000');
INSERT INTO `change_groups` VALUES (10136,10045,9,'2025-11-28 07:39:27.000');
INSERT INTO `change_groups` VALUES (10137,10045,9,'2025-12-04 16:51:29.000');
INSERT INTO `change_groups` VALUES (10138,10045,13,'2026-09-24 10:09:55.000');
INSERT INTO `change_groups` VALUES (10139,10046,5,'2025-11-20 07:03:52.000');
INSERT INTO `change_groups` VALUES (10140,10046,5,'2025-11-20 07:38:28.000');
INSERT INTO `change_groups` VALUES (10141,10046,7,'2025-11-20 10:27:02.000');
INSERT INTO `change_groups` VALUES (10142,10046,7,'2025-11-26 10:05:21.000');
INSERT INTO `change_groups` VALUES (10143,10046,5,'2025-12-05 16:36:59.000');
INSERT INTO `change_groups` VALUES (10144,10046,7,'2025-12-08 17:35:57.000');
INSERT INTO `change_groups` VALUES (10145,10047,9,'2025-11-17 16:49:29.000');
INSERT INTO `change_groups` VALUES (10146,10047,23,'2025-11-28 15:10:47.000');
INSERT INTO `change_groups` VALUES (10147,10048,15,'2025-11-27 18:51:15.000');
INSERT INTO `change_groups` VALUES (10148,10048,22,'2025-11-28 07:24:52.000');
INSERT INTO `change_groups` VALUES (10149,10048,15,'2026-04-12 16:35:10.000');
INSERT INTO `change_groups` VALUES (10150,10049,15,'2025-11-19 09:52:07.000');
INSERT INTO `change_groups` VALUES (10151,10049,9,'2025-11-28 18:15:03.000');
INSERT INTO `change_groups` VALUES (10152,10050,15,'2025-11-19 18:01:19.000');
INSERT INTO `change_groups` VALUES (10153,10050,4,'2025-11-21 09:09:58.000');
INSERT INTO `change_groups` VALUES (10154,10051,15,'2025-11-26 13:03:20.000');
INSERT INTO `change_groups` VALUES (10155,10051,2,'2025-12-03 13:37:46.000');
INSERT INTO `change_groups` VALUES (10156,10051,15,'2025-12-11 17:43:55.000');
INSERT INTO `change_groups` VALUES (10157,10052,7,'2025-11-22 15:28:14.000');
INSERT INTO `change_groups` VALUES (10158,10052,7,'2025-11-24 09:43:06.000');
INSERT INTO `change_groups` VALUES (10159,10052,6,'2025-11-24 14:31:09.000');
INSERT INTO `change_groups` VALUES (10160,10052,11,'2025-11-26 10:53:33.000');
INSERT INTO `change_groups` VALUES (10161,10052,11,'2025-12-03 17:40:47.000');
INSERT INTO `change_groups` VALUES (10162,10052,7,'2025-12-14 12:08:00.000');
INSERT INTO `change_groups` VALUES (10163,10053,7,'2025-11-22 17:00:57.000');
INSERT INTO `change_groups` VALUES (10164,10053,9,'2025-11-26 07:22:11.000');
INSERT INTO `change_groups` VALUES (10165,10054,4,'2025-12-01 08:32:06.000');
INSERT INTO `change_groups` VALUES (10166,10055,17,'2025-11-27 13:53:44.000');
INSERT INTO `change_groups` VALUES (10167,10055,17,'2025-12-04 11:04:22.000');
INSERT INTO `change_groups` VALUES (10168,10056,16,'2025-11-26 09:23:31.000');
INSERT INTO `change_groups` VALUES (10169,10056,16,'2025-11-28 08:15:24.000');
INSERT INTO `change_groups` VALUES (10170,10059,2,'2025-12-01 10:35:46.000');
INSERT INTO `change_groups` VALUES (10171,10059,7,'2025-12-01 10:49:28.000');
INSERT INTO `change_groups` VALUES (10172,10059,16,'2025-12-06 07:31:10.000');
INSERT INTO `change_groups` VALUES (10173,10060,13,'2025-12-01 17:13:31.000');
INSERT INTO `change_groups` VALUES (10174,10060,13,'2025-12-12 13:47:06.000');
INSERT INTO `change_groups` VALUES (10175,10060,16,'2025-12-22 17:00:03.000');
INSERT INTO `change_groups` VALUES (10176,10061,4,'2025-12-14 07:04:56.000');
INSERT INTO `change_groups` VALUES (10177,10061,11,'2025-12-22 16:18:28.000');
INSERT INTO `change_groups` VALUES (10178,10061,6,'2026-01-27 18:56:18.000');
INSERT INTO `change_groups` VALUES (10179,10062,17,'2025-12-05 07:20:53.000');
INSERT INTO `change_groups` VALUES (10180,10062,4,'2025-12-05 16:51:58.000');
INSERT INTO `change_groups` VALUES (10181,10062,17,'2025-12-06 18:11:50.000');
INSERT INTO `change_groups` VALUES (10182,10062,17,'2025-12-07 10:56:27.000');
INSERT INTO `change_groups` VALUES (10183,10062,6,'2025-12-07 12:06:21.000');
INSERT INTO `change_groups` VALUES (10184,10062,17,'2025-12-08 17:00:18.000');
INSERT INTO `change_groups` VALUES (10185,10062,15,'2025-12-10 17:10:39.000');
INSERT INTO `change_groups` VALUES (10186,10062,4,'2025-12-25 11:07:54.000');
INSERT INTO `change_groups` VALUES (10187,10063,3,'2025-12-14 15:24:15.000');
INSERT INTO `change_groups` VALUES (10188,10063,6,'2025-12-20 07:28:12.000');
INSERT INTO `change_groups` VALUES (10189,10063,4,'2025-12-21 07:28:46.000');
INSERT INTO `change_groups` VALUES (10190,10064,16,'2025-12-05 08:12:22.000');
INSERT INTO `change_groups` VALUES (10191,10064,16,'2025-12-08 07:27:23.000');
INSERT INTO `change_groups` VALUES (10192,10064,16,'2025-12-17 14:50:44.000');
INSERT INTO `change_groups` VALUES (10193,10064,16,'2025-12-19 18:17:27.000');
INSERT INTO `change_groups` VALUES (10194,10065,22,'2025-12-08 08:39:37.000');
INSERT INTO `change_groups` VALUES (10195,10065,7,'2025-12-12 16:43:04.000');
INSERT INTO `change_groups` VALUES (10196,10065,9,'2025-12-16 12:11:05.000');
INSERT INTO `change_groups` VALUES (10197,10066,23,'2025-12-08 16:08:21.000');
INSERT INTO `change_groups` VALUES (10198,10066,15,'2026-09-26 07:45:18.000');
INSERT INTO `change_groups` VALUES (10199,10067,23,'2025-12-10 13:19:38.000');
INSERT INTO `change_groups` VALUES (10200,10067,23,'2025-12-14 07:14:50.000');
INSERT INTO `change_groups` VALUES (10201,10068,7,'2025-12-19 07:21:19.000');
INSERT INTO `change_groups` VALUES (10202,10068,5,'2025-12-22 11:17:24.000');
INSERT INTO `change_groups` VALUES (10203,10068,10,'2025-12-24 08:13:26.000');
INSERT INTO `change_groups` VALUES (10204,10068,3,'2025-12-26 15:59:13.000');
INSERT INTO `change_groups` VALUES (10205,10068,7,'2026-03-30 16:24:04.000');
INSERT INTO `change_groups` VALUES (10206,10069,4,'2025-12-10 11:58:29.000');
INSERT INTO `change_groups` VALUES (10207,10069,7,'2025-12-12 15:26:04.000');
INSERT INTO `change_groups` VALUES (10208,10069,11,'2025-12-14 15:40:56.000');
INSERT INTO `change_groups` VALUES (10209,10069,4,'2025-12-22 10:15:37.000');
INSERT INTO `change_groups` VALUES (10210,10069,4,'2025-12-31 11:10:23.000');
INSERT INTO `change_groups` VALUES (10211,10070,11,'2025-12-19 15:00:46.000');
INSERT INTO `change_groups` VALUES (10212,10070,11,'2025-12-22 16:44:49.000');
INSERT INTO `change_groups` VALUES (10213,10071,14,'2025-12-18 17:30:24.000');
INSERT INTO `change_groups` VALUES (10214,10071,10,'2025-12-28 11:24:30.000');
INSERT INTO `change_groups` VALUES (10215,10072,7,'2025-12-13 16:43:35.000');
INSERT INTO `change_groups` VALUES (10216,10072,2,'2025-12-21 18:57:12.000');
INSERT INTO `change_groups` VALUES (10217,10072,17,'2025-12-22 08:46:29.000');
INSERT INTO `change_groups` VALUES (10218,10072,2,'2026-01-02 14:14:36.000');
INSERT INTO `change_groups` VALUES (10219,10072,2,'2026-01-06 07:51:44.000');
INSERT INTO `change_groups` VALUES (10220,10072,2,'2026-02-17 07:16:24.000');
INSERT INTO `change_groups` VALUES (10221,10073,16,'2025-12-13 09:16:49.000');
INSERT INTO `change_groups` VALUES (10222,10073,2,'2025-12-14 10:32:46.000');
INSERT INTO `change_groups` VALUES (10223,10074,17,'2025-12-13 18:33:09.000');
INSERT INTO `change_groups` VALUES (10224,10074,13,'2025-12-14 16:42:54.000');
INSERT INTO `change_groups` VALUES (10225,10074,9,'2025-12-15 07:29:01.000');
INSERT INTO `change_groups` VALUES (10226,10074,15,'2025-12-15 10:49:34.000');
INSERT INTO `change_groups` VALUES (10227,10074,13,'2025-12-24 07:52:13.000');
INSERT INTO `change_groups` VALUES (10228,10076,2,'2025-12-21 10:48:19.000');
INSERT INTO `change_groups` VALUES (10229,10076,3,'2025-12-22 14:21:56.000');
INSERT INTO `change_groups` VALUES (10230,10076,3,'2025-12-23 09:39:25.000');
INSERT INTO `change_groups` VALUES (10231,10076,2,'2025-12-23 17:31:04.000');
INSERT INTO `change_groups` VALUES (10232,10076,5,'2025-12-26 12:58:03.000');
INSERT INTO `change_groups` VALUES (10233,10076,3,'2026-01-03 15:11:25.000');
INSERT INTO `change_groups` VALUES (10234,10077,16,'2025-12-18 15:52:37.000');
INSERT INTO `change_groups` VALUES (10235,10077,16,'2025-12-22 13:15:36.000');
INSERT INTO `change_groups` VALUES (10236,10077,5,'2025-12-25 08:16:55.000');
INSERT INTO `change_groups` VALUES (10237,10077,16,'2026-01-02 08:54:54.000');
INSERT INTO `change_groups` VALUES (10238,10077,4,'2026-04-05 17:33:57.000');
INSERT INTO `change_groups` VALUES (10239,10078,14,'2025-12-19 08:28:43.000');
INSERT INTO `change_groups` VALUES (10240,10079,8,'2025-12-19 14:16:29.000');
INSERT INTO `change_groups` VALUES (10241,10079,8,'2025-12-24 11:14:26.000');
INSERT INTO `change_groups` VALUES (10242,10079,14,'2025-12-30 08:37:25.000');
INSERT INTO `change_groups` VALUES (10243,10079,8,'2026-01-02 08:53:59.000');
INSERT INTO `change_groups` VALUES (10244,10079,8,'2026-01-05 09:29:40.000');
INSERT INTO `change_groups` VALUES (10245,10080,16,'2025-12-30 09:15:59.000');
INSERT INTO `change_groups` VALUES (10246,10080,5,'2026-01-03 11:07:32.000');
INSERT INTO `change_groups` VALUES (10247,10080,7,'2026-02-07 14:49:07.000');
INSERT INTO `change_groups` VALUES (10248,10081,17,'2025-12-23 17:19:18.000');
INSERT INTO `change_groups` VALUES (10249,10081,6,'2025-12-26 16:40:34.000');
INSERT INTO `change_groups` VALUES (10250,10082,14,'2026-01-01 08:25:57.000');
INSERT INTO `change_groups` VALUES (10251,10082,14,'2026-01-01 10:43:19.000');
INSERT INTO `change_groups` VALUES (10252,10082,10,'2026-01-02 12:42:21.000');
INSERT INTO `change_groups` VALUES (10253,10083,2,'2025-12-31 16:22:24.000');
INSERT INTO `change_groups` VALUES (10254,10083,2,'2025-12-31 18:24:50.000');
INSERT INTO `change_groups` VALUES (10255,10083,17,'2026-01-05 11:02:52.000');
INSERT INTO `change_groups` VALUES (10256,10083,7,'2026-01-15 16:18:05.000');
INSERT INTO `change_groups` VALUES (10257,10084,19,'2025-12-31 15:01:46.000');
INSERT INTO `change_groups` VALUES (10258,10084,19,'2026-01-03 12:06:51.000');
INSERT INTO `change_groups` VALUES (10259,10084,11,'2026-01-10 15:30:43.000');
INSERT INTO `change_groups` VALUES (10260,10085,9,'2025-12-30 16:16:09.000');
INSERT INTO `change_groups` VALUES (10261,10085,9,'2026-01-09 11:49:09.000');
INSERT INTO `change_groups` VALUES (10262,10085,11,'2026-01-10 16:32:17.000');
INSERT INTO `change_groups` VALUES (10263,10086,7,'2026-01-02 13:41:39.000');
INSERT INTO `change_groups` VALUES (10264,10086,7,'2026-01-04 12:28:57.000');
INSERT INTO `change_groups` VALUES (10265,10086,7,'2026-01-12 15:08:22.000');
INSERT INTO `change_groups` VALUES (10266,10086,7,'2026-01-20 18:23:09.000');
INSERT INTO `change_groups` VALUES (10267,10087,11,'2026-01-10 14:05:04.000');
INSERT INTO `change_groups` VALUES (10268,10087,2,'2026-01-11 09:50:45.000');
INSERT INTO `change_groups` VALUES (10269,10087,11,'2026-01-18 14:33:48.000');
INSERT INTO `change_groups` VALUES (10270,10088,9,'2026-01-05 15:48:03.000');
INSERT INTO `change_groups` VALUES (10271,10088,9,'2026-09-24 16:13:12.000');
INSERT INTO `change_groups` VALUES (10272,10089,6,'2026-01-11 16:10:37.000');
INSERT INTO `change_groups` VALUES (10273,10089,6,'2026-01-15 07:36:24.000');
INSERT INTO `change_groups` VALUES (10274,10089,10,'2026-01-16 09:18:35.000');
INSERT INTO `change_groups` VALUES (10275,10089,6,'2026-01-25 07:18:11.000');
INSERT INTO `change_groups` VALUES (10276,10090,15,'2026-01-12 09:21:42.000');
INSERT INTO `change_groups` VALUES (10277,10090,3,'2026-01-13 18:47:18.000');
INSERT INTO `change_groups` VALUES (10278,10090,11,'2026-01-20 08:42:06.000');
INSERT INTO `change_groups` VALUES (10279,10090,9,'2026-01-27 12:08:52.000');
INSERT INTO `change_groups` VALUES (10280,10091,6,'2026-01-13 16:24:21.000');
INSERT INTO `change_groups` VALUES (10281,10092,11,'2026-01-15 10:50:22.000');
INSERT INTO `change_groups` VALUES (10282,10092,5,'2026-01-19 15:36:08.000');
INSERT INTO `change_groups` VALUES (10283,10092,15,'2026-01-19 17:42:00.000');
INSERT INTO `change_groups` VALUES (10284,10093,6,'2026-01-20 13:55:29.000');
INSERT INTO `change_groups` VALUES (10285,10093,5,'2026-01-20 18:01:04.000');
INSERT INTO `change_groups` VALUES (10286,10093,4,'2026-01-31 16:13:57.000');
INSERT INTO `change_groups` VALUES (10287,10093,5,'2026-02-16 16:44:25.000');
INSERT INTO `change_groups` VALUES (10288,10094,13,'2026-01-28 10:27:25.000');
INSERT INTO `change_groups` VALUES (10289,10094,9,'2026-01-30 18:23:28.000');
INSERT INTO `change_groups` VALUES (10290,10094,5,'2026-02-01 07:55:18.000');
INSERT INTO `change_groups` VALUES (10291,10094,5,'2026-02-03 09:55:18.000');
INSERT INTO `change_groups` VALUES (10292,10095,8,'2026-01-27 11:09:48.000');
INSERT INTO `change_groups` VALUES (10293,10095,14,'2026-02-03 08:34:30.000');
INSERT INTO `change_groups` VALUES (10294,10095,14,'2026-02-10 14:03:52.000');
INSERT INTO `change_groups` VALUES (10295,10096,15,'2026-02-02 10:29:07.000');
INSERT INTO `change_groups` VALUES (10296,10097,10,'2026-01-29 10:06:25.000');
INSERT INTO `change_groups` VALUES (10297,10097,10,'2026-01-29 11:23:25.000');
INSERT INTO `change_groups` VALUES (10298,10097,14,'2026-02-03 13:07:07.000');
INSERT INTO `change_groups` VALUES (10299,10097,14,'2026-02-11 13:12:07.000');
INSERT INTO `change_groups` VALUES (10300,10098,14,'2026-01-30 16:03:43.000');
INSERT INTO `change_groups` VALUES (10301,10099,21,'2026-02-07 07:52:29.000');
INSERT INTO `change_groups` VALUES (10302,10100,15,'2026-02-02 14:49:53.000');
INSERT INTO `change_groups` VALUES (10303,10100,15,'2026-02-06 12:01:50.000');
INSERT INTO `change_groups` VALUES (10304,10101,9,'2026-01-31 08:44:17.000');
INSERT INTO `change_groups` VALUES (10305,10101,4,'2026-02-04 13:00:35.000');
INSERT INTO `change_groups` VALUES (10306,10101,6,'2026-02-07 15:28:21.000');
INSERT INTO `change_groups` VALUES (10307,10101,7,'2026-02-10 07:59:05.000');
INSERT INTO `change_groups` VALUES (10308,10102,21,'2026-02-03 13:07:03.000');
INSERT INTO `change_groups` VALUES (10309,10102,21,'2026-02-19 11:40:01.000');
INSERT INTO `change_groups` VALUES (10310,10103,17,'2026-02-04 16:46:10.000');
INSERT INTO `change_groups` VALUES (10311,10104,2,'2026-02-17 17:54:25.000');
INSERT INTO `change_groups` VALUES (10312,10104,16,'2026-06-23 17:29:52.000');
INSERT INTO `change_groups` VALUES (10313,10105,11,'2026-02-12 10:41:06.000');
INSERT INTO `change_groups` VALUES (10314,10105,17,'2026-02-24 10:48:17.000');
INSERT INTO `change_groups` VALUES (10315,10106,10,'2026-02-07 16:18:51.000');
INSERT INTO `change_groups` VALUES (10316,10107,4,'2026-05-08 12:58:03.000');
INSERT INTO `change_groups` VALUES (10317,10108,10,'2026-02-04 17:27:29.000');
INSERT INTO `change_groups` VALUES (10318,10108,3,'2026-02-14 09:19:48.000');
INSERT INTO `change_groups` VALUES (10319,10108,13,'2026-02-20 09:58:16.000');
INSERT INTO `change_groups` VALUES (10320,10109,16,'2026-02-12 12:43:48.000');
INSERT INTO `change_groups` VALUES (10321,10109,3,'2026-02-19 13:41:13.000');
INSERT INTO `change_groups` VALUES (10322,10109,11,'2026-02-20 08:30:34.000');
INSERT INTO `change_groups` VALUES (10323,10109,11,'2026-03-02 08:59:00.000');
INSERT INTO `change_groups` VALUES (10324,10110,17,'2026-02-08 13:24:51.000');
INSERT INTO `change_groups` VALUES (10325,10111,17,'2026-02-18 09:12:35.000');
INSERT INTO `change_groups` VALUES (10326,10112,5,'2026-02-16 12:58:10.000');
INSERT INTO `change_groups` VALUES (10327,10113,9,'2026-02-17 18:58:40.000');
INSERT INTO `change_groups` VALUES (10328,10113,11,'2026-02-22 13:51:01.000');
INSERT INTO `change_groups` VALUES (10329,10114,10,'2026-02-14 10:39:19.000');
INSERT INTO `change_groups` VALUES (10330,10114,10,'2026-02-20 12:20:39.000');
INSERT INTO `change_groups` VALUES (10331,10114,10,'2026-02-22 08:00:33.000');
INSERT INTO `change_groups` VALUES (10332,10114,10,'2026-02-24 17:19:03.000');
INSERT INTO `change_groups` VALUES (10333,10114,10,'2026-03-01 14:15:39.000');
INSERT INTO `change_groups` VALUES (10334,10114,10,'2026-03-04 07:48:17.000');
INSERT INTO `change_groups` VALUES (10335,10115,14,'2026-03-05 11:26:05.000');
INSERT INTO `change_groups` VALUES (10336,10116,15,'2026-02-15 10:05:27.000');
INSERT INTO `change_groups` VALUES (10337,10116,7,'2026-02-21 18:12:21.000');
INSERT INTO `change_groups` VALUES (10338,10116,7,'2026-02-25 08:42:44.000');
INSERT INTO `change_groups` VALUES (10339,10116,11,'2026-03-09 09:18:21.000');
INSERT INTO `change_groups` VALUES (10340,10117,15,'2026-03-08 09:30:01.000');
INSERT INTO `change_groups` VALUES (10341,10117,3,'2026-06-03 17:40:49.000');
INSERT INTO `change_groups` VALUES (10342,10118,7,'2026-03-20 07:13:44.000');
INSERT INTO `change_groups` VALUES (10343,10119,16,'2026-02-23 15:16:59.000');
INSERT INTO `change_groups` VALUES (10344,10119,15,'2026-02-25 07:56:05.000');
INSERT INTO `change_groups` VALUES (10345,10119,6,'2026-03-04 13:44:39.000');
INSERT INTO `change_groups` VALUES (10346,10119,6,'2026-03-13 14:40:43.000');
INSERT INTO `change_groups` VALUES (10347,10120,2,'2026-02-27 07:38:44.000');
INSERT INTO `change_groups` VALUES (10348,10120,13,'2026-03-02 13:39:55.000');
INSERT INTO `change_groups` VALUES (10349,10120,9,'2026-03-06 08:32:36.000');
INSERT INTO `change_groups` VALUES (10350,10121,23,'2026-02-24 17:16:38.000');
INSERT INTO `change_groups` VALUES (10351,10121,7,'2026-03-02 09:06:43.000');
INSERT INTO `change_groups` VALUES (10352,10121,7,'2026-03-20 18:51:15.000');
INSERT INTO `change_groups` VALUES (10353,10122,5,'2026-02-26 18:55:27.000');
INSERT INTO `change_groups` VALUES (10354,10122,11,'2026-03-01 15:35:13.000');
INSERT INTO `change_groups` VALUES (10355,10123,15,'2026-02-26 10:38:05.000');
INSERT INTO `change_groups` VALUES (10356,10123,21,'2026-02-26 18:28:01.000');
INSERT INTO `change_groups` VALUES (10357,10124,15,'2026-03-19 10:14:15.000');
INSERT INTO `change_groups` VALUES (10358,10125,22,'2026-03-06 15:11:31.000');
INSERT INTO `change_groups` VALUES (10359,10125,7,'2026-03-13 16:23:00.000');
INSERT INTO `change_groups` VALUES (10360,10126,4,'2026-03-06 09:23:47.000');
INSERT INTO `change_groups` VALUES (10361,10126,17,'2026-03-13 15:49:56.000');
INSERT INTO `change_groups` VALUES (10362,10127,11,'2026-03-10 14:36:22.000');
INSERT INTO `change_groups` VALUES (10363,10127,4,'2026-03-13 09:53:23.000');
INSERT INTO `change_groups` VALUES (10364,10128,7,'2026-03-06 13:58:07.000');
INSERT INTO `change_groups` VALUES (10365,10128,7,'2026-03-08 15:39:18.000');
INSERT INTO `change_groups` VALUES (10366,10129,17,'2026-03-10 17:53:53.000');
INSERT INTO `change_groups` VALUES (10367,10130,3,'2026-03-02 16:24:14.000');
INSERT INTO `change_groups` VALUES (10368,10130,4,'2026-03-03 13:25:46.000');
INSERT INTO `change_groups` VALUES (10369,10130,3,'2026-03-13 10:16:38.000');
INSERT INTO `change_groups` VALUES (10370,10131,5,'2026-03-03 14:09:52.000');
INSERT INTO `change_groups` VALUES (10371,10131,5,'2026-03-06 14:50:21.000');
INSERT INTO `change_groups` VALUES (10372,10131,11,'2026-03-10 10:35:04.000');
INSERT INTO `change_groups` VALUES (10373,10132,23,'2026-03-22 07:58:44.000');
INSERT INTO `change_groups` VALUES (10374,10133,13,'2026-03-12 15:05:31.000');
INSERT INTO `change_groups` VALUES (10375,10133,6,'2026-03-13 10:22:44.000');
INSERT INTO `change_groups` VALUES (10376,10133,6,'2026-03-17 07:41:22.000');
INSERT INTO `change_groups` VALUES (10377,10133,6,'2026-03-17 18:44:54.000');
INSERT INTO `change_groups` VALUES (10378,10133,13,'2026-03-21 16:29:22.000');
INSERT INTO `change_groups` VALUES (10379,10133,13,'2026-03-24 09:05:50.000');
INSERT INTO `change_groups` VALUES (10380,10133,7,'2026-03-31 09:01:43.000');
INSERT INTO `change_groups` VALUES (10381,10134,15,'2026-03-12 16:55:23.000');
INSERT INTO `change_groups` VALUES (10382,10134,5,'2026-03-14 18:19:14.000');
INSERT INTO `change_groups` VALUES (10383,10134,16,'2026-03-29 14:21:25.000');
INSERT INTO `change_groups` VALUES (10384,10135,3,'2026-03-13 13:39:06.000');
INSERT INTO `change_groups` VALUES (10385,10135,6,'2026-03-23 12:09:12.000');
INSERT INTO `change_groups` VALUES (10386,10135,11,'2026-04-01 12:41:24.000');
INSERT INTO `change_groups` VALUES (10387,10136,5,'2026-03-16 13:33:36.000');
INSERT INTO `change_groups` VALUES (10388,10136,11,'2026-03-23 15:16:59.000');
INSERT INTO `change_groups` VALUES (10389,10136,9,'2026-03-25 10:38:20.000');
INSERT INTO `change_groups` VALUES (10390,10138,5,'2026-03-15 09:28:32.000');
INSERT INTO `change_groups` VALUES (10391,10138,5,'2026-03-16 17:50:27.000');
INSERT INTO `change_groups` VALUES (10392,10138,9,'2026-03-19 12:25:05.000');
INSERT INTO `change_groups` VALUES (10393,10138,5,'2026-03-23 14:50:31.000');
INSERT INTO `change_groups` VALUES (10394,10138,5,'2026-03-27 16:12:52.000');
INSERT INTO `change_groups` VALUES (10395,10138,5,'2026-07-27 08:40:34.000');
INSERT INTO `change_groups` VALUES (10396,10139,13,'2026-03-18 15:05:37.000');
INSERT INTO `change_groups` VALUES (10397,10139,16,'2026-03-24 08:50:48.000');
INSERT INTO `change_groups` VALUES (10398,10139,16,'2026-03-29 11:26:24.000');
INSERT INTO `change_groups` VALUES (10399,10139,16,'2026-04-05 17:59:37.000');
INSERT INTO `change_groups` VALUES (10400,10140,2,'2026-03-19 14:09:44.000');
INSERT INTO `change_groups` VALUES (10401,10141,5,'2026-03-21 07:13:55.000');
INSERT INTO `change_groups` VALUES (10402,10141,9,'2026-03-31 13:20:14.000');
INSERT INTO `change_groups` VALUES (10403,10141,9,'2026-04-01 12:58:14.000');
INSERT INTO `change_groups` VALUES (10404,10141,5,'2026-04-09 06:59:27.000');
INSERT INTO `change_groups` VALUES (10405,10141,6,'2026-04-09 17:34:58.000');
INSERT INTO `change_groups` VALUES (10406,10141,5,'2026-05-28 16:18:29.000');
INSERT INTO `change_groups` VALUES (10407,10141,7,'2026-06-07 14:54:46.000');
INSERT INTO `change_groups` VALUES (10408,10143,9,'2026-03-26 11:16:48.000');
INSERT INTO `change_groups` VALUES (10409,10143,22,'2026-04-01 15:26:58.000');
INSERT INTO `change_groups` VALUES (10410,10144,11,'2026-03-22 13:16:27.000');
INSERT INTO `change_groups` VALUES (10411,10144,15,'2026-03-26 14:07:10.000');
INSERT INTO `change_groups` VALUES (10412,10144,11,'2026-03-31 06:01:17.000');
INSERT INTO `change_groups` VALUES (10413,10145,11,'2026-03-23 17:58:17.000');
INSERT INTO `change_groups` VALUES (10414,10145,17,'2026-03-30 09:55:40.000');
INSERT INTO `change_groups` VALUES (10415,10145,6,'2026-04-05 13:38:45.000');
INSERT INTO `change_groups` VALUES (10416,10145,6,'2026-04-11 09:49:49.000');
INSERT INTO `change_groups` VALUES (10417,10146,15,'2026-03-28 12:34:31.000');
INSERT INTO `change_groups` VALUES (10418,10146,13,'2026-03-29 14:15:13.000');
INSERT INTO `change_groups` VALUES (10419,10147,4,'2026-03-27 07:02:02.000');
INSERT INTO `change_groups` VALUES (10420,10147,5,'2026-03-28 08:10:51.000');
INSERT INTO `change_groups` VALUES (10421,10147,10,'2026-03-29 12:24:30.000');
INSERT INTO `change_groups` VALUES (10422,10147,17,'2026-04-02 08:11:36.000');
INSERT INTO `change_groups` VALUES (10423,10147,10,'2026-04-07 16:55:56.000');
INSERT INTO `change_groups` VALUES (10424,10147,10,'2026-04-10 09:44:21.000');
INSERT INTO `change_groups` VALUES (10425,10147,17,'2026-04-19 08:10:11.000');
INSERT INTO `change_groups` VALUES (10426,10148,11,'2026-04-09 13:00:25.000');
INSERT INTO `change_groups` VALUES (10427,10148,21,'2026-04-10 15:25:27.000');
INSERT INTO `change_groups` VALUES (10428,10148,7,'2026-04-17 17:11:36.000');
INSERT INTO `change_groups` VALUES (10429,10148,9,'2026-04-29 06:50:55.000');
INSERT INTO `change_groups` VALUES (10430,10149,16,'2026-04-07 12:36:07.000');
INSERT INTO `change_groups` VALUES (10431,10149,7,'2026-04-10 17:49:23.000');
INSERT INTO `change_groups` VALUES (10432,10149,7,'2026-04-13 17:15:26.000');
INSERT INTO `change_groups` VALUES (10433,10149,7,'2026-04-30 08:56:44.000');
INSERT INTO `change_groups` VALUES (10434,10150,10,'2026-04-04 17:29:44.000');
INSERT INTO `change_groups` VALUES (10435,10150,10,'2026-04-15 12:38:01.000');
INSERT INTO `change_groups` VALUES (10436,10150,10,'2026-04-26 15:02:13.000');
INSERT INTO `change_groups` VALUES (10437,10151,10,'2026-04-11 10:04:41.000');
INSERT INTO `change_groups` VALUES (10438,10151,14,'2026-05-05 11:11:24.000');
INSERT INTO `change_groups` VALUES (10439,10152,10,'2026-04-11 16:38:57.000');
INSERT INTO `change_groups` VALUES (10440,10152,10,'2026-04-26 17:15:54.000');
INSERT INTO `change_groups` VALUES (10441,10153,2,'2026-04-20 14:07:31.000');
INSERT INTO `change_groups` VALUES (10442,10153,17,'2026-04-22 10:28:25.000');
INSERT INTO `change_groups` VALUES (10443,10153,15,'2026-04-25 11:19:10.000');
INSERT INTO `change_groups` VALUES (10444,10153,11,'2026-04-27 10:20:26.000');
INSERT INTO `change_groups` VALUES (10445,10154,17,'2026-04-15 12:35:48.000');
INSERT INTO `change_groups` VALUES (10446,10154,17,'2026-04-18 07:41:31.000');
INSERT INTO `change_groups` VALUES (10447,10154,17,'2026-04-18 10:40:18.000');
INSERT INTO `change_groups` VALUES (10448,10154,17,'2026-04-25 06:28:31.000');
INSERT INTO `change_groups` VALUES (10449,10155,17,'2026-04-17 17:14:06.000');
INSERT INTO `change_groups` VALUES (10450,10155,17,'2026-04-18 17:55:53.000');
INSERT INTO `change_groups` VALUES (10451,10155,10,'2026-04-24 13:07:35.000');
INSERT INTO `change_groups` VALUES (10452,10156,6,'2026-04-22 07:47:48.000');
INSERT INTO `change_groups` VALUES (10453,10156,2,'2026-04-24 06:01:45.000');
INSERT INTO `change_groups` VALUES (10454,10156,6,'2026-04-25 14:49:32.000');
INSERT INTO `change_groups` VALUES (10455,10156,6,'2026-04-28 15:35:58.000');
INSERT INTO `change_groups` VALUES (10456,10157,8,'2026-04-20 15:23:15.000');
INSERT INTO `change_groups` VALUES (10457,10157,10,'2026-04-22 08:25:38.000');
INSERT INTO `change_groups` VALUES (10458,10157,8,'2026-04-22 13:37:03.000');
INSERT INTO `change_groups` VALUES (10459,10158,17,'2026-05-01 11:25:13.000');
INSERT INTO `change_groups` VALUES (10460,10158,13,'2026-05-13 14:42:52.000');
INSERT INTO `change_groups` VALUES (10461,10160,13,'2026-04-26 10:47:03.000');
INSERT INTO `change_groups` VALUES (10462,10160,11,'2026-04-26 16:41:13.000');
INSERT INTO `change_groups` VALUES (10463,10160,11,'2026-05-05 13:14:39.000');
INSERT INTO `change_groups` VALUES (10464,10161,4,'2026-04-24 17:34:00.000');
INSERT INTO `change_groups` VALUES (10465,10161,11,'2026-04-27 10:11:09.000');
INSERT INTO `change_groups` VALUES (10466,10161,17,'2026-05-04 09:46:56.000');
INSERT INTO `change_groups` VALUES (10467,10162,22,'2026-04-23 07:18:15.000');
INSERT INTO `change_groups` VALUES (10468,10162,22,'2026-04-29 12:20:02.000');
INSERT INTO `change_groups` VALUES (10469,10162,21,'2026-05-13 15:29:51.000');
INSERT INTO `change_groups` VALUES (10470,10163,15,'2026-04-19 11:59:30.000');
INSERT INTO `change_groups` VALUES (10471,10163,9,'2026-04-19 15:07:17.000');
INSERT INTO `change_groups` VALUES (10472,10163,2,'2026-04-22 13:40:32.000');
INSERT INTO `change_groups` VALUES (10473,10163,6,'2026-04-24 14:44:41.000');
INSERT INTO `change_groups` VALUES (10474,10163,2,'2026-05-02 08:11:57.000');
INSERT INTO `change_groups` VALUES (10475,10164,4,'2026-04-30 11:10:27.000');
INSERT INTO `change_groups` VALUES (10476,10164,2,'2026-05-01 17:50:45.000');
INSERT INTO `change_groups` VALUES (10477,10165,15,'2026-04-28 07:58:10.000');
INSERT INTO `change_groups` VALUES (10478,10165,6,'2026-05-01 08:30:01.000');
INSERT INTO `change_groups` VALUES (10479,10165,6,'2026-05-04 09:16:10.000');
INSERT INTO `change_groups` VALUES (10480,10165,11,'2026-05-07 10:42:18.000');
INSERT INTO `change_groups` VALUES (10481,10166,7,'2026-04-28 07:56:56.000');
INSERT INTO `change_groups` VALUES (10482,10166,7,'2026-04-28 11:30:53.000');
INSERT INTO `change_groups` VALUES (10483,10167,17,'2026-04-23 09:32:42.000');
INSERT INTO `change_groups` VALUES (10484,10167,17,'2026-05-03 06:32:07.000');
INSERT INTO `change_groups` VALUES (10485,10167,7,'2026-05-07 11:02:36.000');
INSERT INTO `change_groups` VALUES (10486,10167,4,'2026-05-08 08:24:09.000');
INSERT INTO `change_groups` VALUES (10487,10168,9,'2026-05-03 06:12:34.000');
INSERT INTO `change_groups` VALUES (10488,10169,2,'2026-04-24 13:01:54.000');
INSERT INTO `change_groups` VALUES (10489,10169,13,'2026-05-03 12:41:05.000');
INSERT INTO `change_groups` VALUES (10490,10169,3,'2026-05-04 16:27:39.000');
INSERT INTO `change_groups` VALUES (10491,10169,3,'2026-05-07 06:42:50.000');
INSERT INTO `change_groups` VALUES (10492,10170,13,'2026-04-27 12:20:38.000');
INSERT INTO `change_groups` VALUES (10493,10171,14,'2026-05-02 10:03:40.000');
INSERT INTO `change_groups` VALUES (10494,10171,14,'2026-05-03 12:36:53.000');
INSERT INTO `change_groups` VALUES (10495,10171,10,'2026-05-03 12:39:07.000');
INSERT INTO `change_groups` VALUES (10496,10171,8,'2026-05-04 06:27:09.000');
INSERT INTO `change_groups` VALUES (10497,10171,14,'2026-05-12 08:37:13.000');
INSERT INTO `change_groups` VALUES (10498,10174,2,'2026-05-11 06:37:57.000');
INSERT INTO `change_groups` VALUES (10499,10174,2,'2026-05-14 13:59:23.000');
INSERT INTO `change_groups` VALUES (10500,10175,21,'2026-05-07 16:52:32.000');
INSERT INTO `change_groups` VALUES (10501,10175,7,'2026-05-08 12:44:49.000');
INSERT INTO `change_groups` VALUES (10502,10175,7,'2026-05-14 14:45:42.000');
INSERT INTO `change_groups` VALUES (10503,10176,9,'2026-05-16 07:27:49.000');
INSERT INTO `change_groups` VALUES (10504,10177,23,'2026-05-09 11:31:19.000');
INSERT INTO `change_groups` VALUES (10505,10177,7,'2026-05-14 14:54:04.000');
INSERT INTO `change_groups` VALUES (10506,10178,4,'2026-05-20 08:27:49.000');
INSERT INTO `change_groups` VALUES (10507,10178,7,'2026-05-23 12:32:45.000');
INSERT INTO `change_groups` VALUES (10508,10178,2,'2026-06-07 07:10:50.000');
INSERT INTO `change_groups` VALUES (10509,10179,4,'2026-05-22 07:52:20.000');
INSERT INTO `change_groups` VALUES (10510,10180,10,'2026-05-19 06:03:41.000');
INSERT INTO `change_groups` VALUES (10511,10180,10,'2026-05-19 13:30:49.000');
INSERT INTO `change_groups` VALUES (10512,10180,14,'2026-05-22 15:31:50.000');
INSERT INTO `change_groups` VALUES (10513,10180,14,'2026-05-24 14:28:45.000');
INSERT INTO `change_groups` VALUES (10514,10180,8,'2026-05-29 13:45:24.000');
INSERT INTO `change_groups` VALUES (10515,10181,7,'2026-05-24 13:26:09.000');
INSERT INTO `change_groups` VALUES (10516,10181,16,'2026-06-07 11:09:15.000');
INSERT INTO `change_groups` VALUES (10517,10183,4,'2026-05-17 16:29:30.000');
INSERT INTO `change_groups` VALUES (10518,10183,17,'2026-05-21 12:24:21.000');
INSERT INTO `change_groups` VALUES (10519,10183,7,'2026-05-22 13:00:49.000');
INSERT INTO `change_groups` VALUES (10520,10183,4,'2026-06-01 07:47:46.000');
INSERT INTO `change_groups` VALUES (10521,10184,7,'2026-05-14 15:27:16.000');
INSERT INTO `change_groups` VALUES (10522,10184,16,'2026-05-17 08:43:48.000');
INSERT INTO `change_groups` VALUES (10523,10184,7,'2026-05-25 14:33:09.000');
INSERT INTO `change_groups` VALUES (10524,10184,11,'2026-06-02 14:25:36.000');
INSERT INTO `change_groups` VALUES (10525,10184,9,'2026-09-15 11:00:59.000');
INSERT INTO `change_groups` VALUES (10526,10185,5,'2026-05-14 15:40:50.000');
INSERT INTO `change_groups` VALUES (10527,10185,18,'2026-05-14 17:12:46.000');
INSERT INTO `change_groups` VALUES (10528,10185,15,'2026-05-22 09:46:16.000');
INSERT INTO `change_groups` VALUES (10529,10185,5,'2026-05-24 08:25:11.000');
INSERT INTO `change_groups` VALUES (10530,10185,5,'2026-05-24 11:09:03.000');
INSERT INTO `change_groups` VALUES (10531,10186,3,'2026-05-24 11:54:29.000');
INSERT INTO `change_groups` VALUES (10532,10186,15,'2026-05-24 12:40:00.000');
INSERT INTO `change_groups` VALUES (10533,10186,9,'2026-05-30 07:20:32.000');
INSERT INTO `change_groups` VALUES (10534,10187,19,'2026-05-17 06:53:43.000');
INSERT INTO `change_groups` VALUES (10535,10187,19,'2026-05-18 08:41:56.000');
INSERT INTO `change_groups` VALUES (10536,10187,21,'2026-05-26 11:20:53.000');
INSERT INTO `change_groups` VALUES (10537,10188,17,'2026-05-25 10:43:33.000');
INSERT INTO `change_groups` VALUES (10538,10188,11,'2026-06-05 07:10:56.000');
INSERT INTO `change_groups` VALUES (10539,10188,4,'2026-06-11 13:45:06.000');
INSERT INTO `change_groups` VALUES (10540,10189,2,'2026-05-19 12:25:08.000');
INSERT INTO `change_groups` VALUES (10541,10189,9,'2026-05-21 06:03:05.000');
INSERT INTO `change_groups` VALUES (10542,10189,17,'2026-05-27 06:18:21.000');
INSERT INTO `change_groups` VALUES (10543,10189,3,'2026-05-30 14:41:16.000');
INSERT INTO `change_groups` VALUES (10544,10189,16,'2026-06-10 11:41:22.000');
INSERT INTO `change_groups` VALUES (10545,10190,5,'2026-05-28 13:55:33.000');
INSERT INTO `change_groups` VALUES (10546,10190,6,'2026-06-11 08:13:21.000');
INSERT INTO `change_groups` VALUES (10547,10191,11,'2026-05-22 06:32:26.000');
INSERT INTO `change_groups` VALUES (10548,10191,21,'2026-05-24 09:05:39.000');
INSERT INTO `change_groups` VALUES (10549,10192,16,'2026-05-20 10:07:09.000');
INSERT INTO `change_groups` VALUES (10550,10193,4,'2026-05-21 16:34:22.000');
INSERT INTO `change_groups` VALUES (10551,10194,8,'2026-06-06 12:53:53.000');
INSERT INTO `change_groups` VALUES (10552,10195,5,'2026-05-21 17:43:57.000');
INSERT INTO `change_groups` VALUES (10553,10195,3,'2026-05-23 15:21:01.000');
INSERT INTO `change_groups` VALUES (10554,10195,10,'2026-05-24 11:48:22.000');
INSERT INTO `change_groups` VALUES (10555,10195,10,'2026-05-26 16:50:20.000');
INSERT INTO `change_groups` VALUES (10556,10195,10,'2026-06-08 11:01:09.000');
INSERT INTO `change_groups` VALUES (10557,10195,10,'2026-06-10 14:41:43.000');
INSERT INTO `change_groups` VALUES (10558,10196,6,'2026-05-22 17:57:50.000');
INSERT INTO `change_groups` VALUES (10559,10196,11,'2026-05-24 14:16:24.000');
INSERT INTO `change_groups` VALUES (10560,10196,6,'2026-06-03 16:33:09.000');
INSERT INTO `change_groups` VALUES (10561,10196,11,'2026-06-09 09:39:48.000');
INSERT INTO `change_groups` VALUES (10562,10196,5,'2026-06-14 16:32:03.000');
INSERT INTO `change_groups` VALUES (10563,10198,3,'2026-05-24 14:54:32.000');
INSERT INTO `change_groups` VALUES (10564,10198,2,'2026-05-25 08:58:41.000');
INSERT INTO `change_groups` VALUES (10565,10198,7,'2026-05-30 09:37:27.000');
INSERT INTO `change_groups` VALUES (10566,10199,2,'2026-05-27 10:38:05.000');
INSERT INTO `change_groups` VALUES (10567,10199,11,'2026-05-29 10:40:40.000');
INSERT INTO `change_groups` VALUES (10568,10199,11,'2026-06-06 15:21:40.000');
INSERT INTO `change_groups` VALUES (10569,10200,17,'2026-06-01 14:14:31.000');
INSERT INTO `change_groups` VALUES (10570,10200,2,'2026-06-03 13:51:50.000');
INSERT INTO `change_groups` VALUES (10571,10200,2,'2026-06-06 12:48:25.000');
INSERT INTO `change_groups` VALUES (10572,10200,3,'2026-06-16 08:13:15.000');
INSERT INTO `change_groups` VALUES (10573,10201,9,'2026-06-01 15:44:26.000');
INSERT INTO `change_groups` VALUES (10574,10201,11,'2026-06-07 17:24:42.000');
INSERT INTO `change_groups` VALUES (10575,10201,17,'2026-06-13 17:06:01.000');
INSERT INTO `change_groups` VALUES (10576,10201,9,'2026-06-17 10:29:04.000');
INSERT INTO `change_groups` VALUES (10577,10202,5,'2026-05-26 13:45:59.000');
INSERT INTO `change_groups` VALUES (10578,10202,5,'2026-05-26 14:50:12.000');
INSERT INTO `change_groups` VALUES (10579,10202,4,'2026-05-31 17:04:49.000');
INSERT INTO `change_groups` VALUES (10580,10202,3,'2026-06-01 09:07:15.000');
INSERT INTO `change_groups` VALUES (10581,10202,15,'2026-06-05 09:53:17.000');
INSERT INTO `change_groups` VALUES (10582,10202,5,'2026-06-11 15:10:53.000');
INSERT INTO `change_groups` VALUES (10583,10203,14,'2026-05-30 09:34:25.000');
INSERT INTO `change_groups` VALUES (10584,10203,10,'2026-06-05 07:56:49.000');
INSERT INTO `change_groups` VALUES (10585,10203,14,'2026-06-05 08:41:27.000');
INSERT INTO `change_groups` VALUES (10586,10204,11,'2026-05-29 15:18:52.000');
INSERT INTO `change_groups` VALUES (10587,10204,11,'2026-06-06 08:24:02.000');
INSERT INTO `change_groups` VALUES (10588,10204,9,'2026-07-08 11:27:20.000');
INSERT INTO `change_groups` VALUES (10589,10206,7,'2026-06-01 13:43:52.000');
INSERT INTO `change_groups` VALUES (10590,10207,17,'2026-06-03 15:06:31.000');
INSERT INTO `change_groups` VALUES (10591,10207,17,'2026-06-05 09:02:46.000');
INSERT INTO `change_groups` VALUES (10592,10207,9,'2026-06-10 10:24:50.000');
INSERT INTO `change_groups` VALUES (10593,10208,17,'2026-06-03 13:17:19.000');
INSERT INTO `change_groups` VALUES (10594,10208,2,'2026-06-08 13:30:38.000');
INSERT INTO `change_groups` VALUES (10595,10208,13,'2026-06-12 13:51:15.000');
INSERT INTO `change_groups` VALUES (10596,10210,17,'2026-06-17 17:10:40.000');
INSERT INTO `change_groups` VALUES (10597,10211,6,'2026-06-06 17:13:37.000');
INSERT INTO `change_groups` VALUES (10598,10211,11,'2026-06-07 15:22:55.000');
INSERT INTO `change_groups` VALUES (10599,10211,9,'2026-07-06 11:15:28.000');
INSERT INTO `change_groups` VALUES (10600,10212,13,'2026-06-12 08:01:59.000');
INSERT INTO `change_groups` VALUES (10601,10212,9,'2026-06-18 17:26:41.000');
INSERT INTO `change_groups` VALUES (10602,10212,9,'2026-06-22 10:37:42.000');
INSERT INTO `change_groups` VALUES (10603,10212,13,'2026-06-28 06:58:31.000');
INSERT INTO `change_groups` VALUES (10604,10213,11,'2026-06-25 17:39:49.000');
INSERT INTO `change_groups` VALUES (10605,10214,11,'2026-06-18 15:26:01.000');
INSERT INTO `change_groups` VALUES (10606,10215,19,'2026-06-13 09:41:55.000');
INSERT INTO `change_groups` VALUES (10607,10215,6,'2026-06-20 13:03:06.000');
INSERT INTO `change_groups` VALUES (10608,10215,6,'2026-06-26 11:37:18.000');
INSERT INTO `change_groups` VALUES (10609,10215,15,'2026-07-07 06:23:08.000');
INSERT INTO `change_groups` VALUES (10610,10216,15,'2026-06-17 09:15:05.000');
INSERT INTO `change_groups` VALUES (10611,10217,6,'2026-06-17 15:25:05.000');
INSERT INTO `change_groups` VALUES (10612,10218,6,'2026-06-23 08:33:24.000');
INSERT INTO `change_groups` VALUES (10613,10218,6,'2026-06-24 13:49:54.000');
INSERT INTO `change_groups` VALUES (10614,10218,5,'2026-06-24 16:01:26.000');
INSERT INTO `change_groups` VALUES (10615,10219,15,'2026-06-18 16:23:22.000');
INSERT INTO `change_groups` VALUES (10616,10219,15,'2026-06-20 12:49:09.000');
INSERT INTO `change_groups` VALUES (10617,10219,15,'2026-07-06 08:43:08.000');
INSERT INTO `change_groups` VALUES (10618,10220,11,'2026-06-19 12:10:06.000');
INSERT INTO `change_groups` VALUES (10619,10220,7,'2026-06-22 14:05:26.000');
INSERT INTO `change_groups` VALUES (10620,10220,9,'2026-07-03 10:21:32.000');
INSERT INTO `change_groups` VALUES (10621,10220,11,'2026-07-03 12:55:23.000');
INSERT INTO `change_groups` VALUES (10622,10220,11,'2026-07-05 16:47:57.000');
INSERT INTO `change_groups` VALUES (10623,10221,3,'2026-06-18 08:15:18.000');
INSERT INTO `change_groups` VALUES (10624,10221,3,'2026-06-18 17:54:42.000');
INSERT INTO `change_groups` VALUES (10625,10221,3,'2026-07-04 15:44:52.000');
INSERT INTO `change_groups` VALUES (10626,10222,4,'2026-06-27 12:31:05.000');
INSERT INTO `change_groups` VALUES (10627,10222,4,'2026-07-01 08:51:27.000');
INSERT INTO `change_groups` VALUES (10628,10222,4,'2026-07-13 14:56:39.000');
INSERT INTO `change_groups` VALUES (10629,10223,2,'2026-06-21 12:16:18.000');
INSERT INTO `change_groups` VALUES (10630,10223,11,'2026-06-23 07:21:10.000');
INSERT INTO `change_groups` VALUES (10631,10223,2,'2026-06-24 10:02:08.000');
INSERT INTO `change_groups` VALUES (10632,10223,2,'2026-07-04 07:03:03.000');
INSERT INTO `change_groups` VALUES (10633,10223,2,'2026-07-08 10:20:56.000');
INSERT INTO `change_groups` VALUES (10634,10224,23,'2026-06-23 13:41:43.000');
INSERT INTO `change_groups` VALUES (10635,10224,23,'2026-06-24 16:04:42.000');
INSERT INTO `change_groups` VALUES (10636,10224,11,'2026-06-26 14:21:08.000');
INSERT INTO `change_groups` VALUES (10637,10224,23,'2026-06-28 09:57:20.000');
INSERT INTO `change_groups` VALUES (10638,10225,14,'2026-06-21 15:38:13.000');
INSERT INTO `change_groups` VALUES (10639,10225,14,'2026-07-01 14:17:40.000');
INSERT INTO `change_groups` VALUES (10640,10225,8,'2026-07-10 10:10:24.000');
INSERT INTO `change_groups` VALUES (10641,10225,8,'2026-07-12 11:54:31.000');
INSERT INTO `change_groups` VALUES (10642,10225,8,'2026-07-19 15:52:59.000');
INSERT INTO `change_groups` VALUES (10643,10226,21,'2026-06-29 07:06:52.000');
INSERT INTO `change_groups` VALUES (10644,10226,15,'2026-07-01 12:42:46.000');
INSERT INTO `change_groups` VALUES (10645,10226,22,'2026-07-10 09:59:37.000');
INSERT INTO `change_groups` VALUES (10646,10227,7,'2026-06-30 13:16:00.000');
INSERT INTO `change_groups` VALUES (10647,10227,21,'2026-07-07 11:43:05.000');
INSERT INTO `change_groups` VALUES (10648,10227,21,'2026-07-10 13:47:25.000');
INSERT INTO `change_groups` VALUES (10649,10227,7,'2026-07-20 16:17:52.000');
INSERT INTO `change_groups` VALUES (10650,10228,9,'2026-06-23 13:52:11.000');
INSERT INTO `change_groups` VALUES (10651,10228,3,'2026-07-11 15:14:49.000');
INSERT INTO `change_groups` VALUES (10652,10229,15,'2026-07-01 10:52:32.000');
INSERT INTO `change_groups` VALUES (10653,10229,12,'2026-07-04 07:29:52.000');
INSERT INTO `change_groups` VALUES (10654,10229,4,'2026-07-04 08:55:34.000');
INSERT INTO `change_groups` VALUES (10655,10230,11,'2026-07-07 08:14:07.000');
INSERT INTO `change_groups` VALUES (10656,10230,17,'2026-07-08 10:27:07.000');
INSERT INTO `change_groups` VALUES (10657,10230,17,'2026-07-12 06:25:15.000');
INSERT INTO `change_groups` VALUES (10658,10232,5,'2026-07-07 14:24:22.000');
INSERT INTO `change_groups` VALUES (10659,10232,3,'2026-07-09 12:41:12.000');
INSERT INTO `change_groups` VALUES (10660,10233,21,'2026-07-12 07:47:13.000');
INSERT INTO `change_groups` VALUES (10661,10233,21,'2026-07-18 14:08:23.000');
INSERT INTO `change_groups` VALUES (10662,10233,21,'2026-07-22 07:23:06.000');
INSERT INTO `change_groups` VALUES (10663,10234,10,'2026-07-19 08:36:46.000');
INSERT INTO `change_groups` VALUES (10664,10234,10,'2026-07-29 08:21:46.000');
INSERT INTO `change_groups` VALUES (10665,10235,11,'2026-07-21 14:44:36.000');
INSERT INTO `change_groups` VALUES (10666,10236,9,'2026-07-13 10:37:13.000');
INSERT INTO `change_groups` VALUES (10667,10236,7,'2026-07-15 08:21:02.000');
INSERT INTO `change_groups` VALUES (10668,10236,17,'2026-07-21 14:53:58.000');
INSERT INTO `change_groups` VALUES (10669,10236,17,'2026-07-22 12:28:56.000');
INSERT INTO `change_groups` VALUES (10670,10237,16,'2026-07-11 13:18:33.000');
INSERT INTO `change_groups` VALUES (10671,10237,11,'2026-07-16 11:26:40.000');
INSERT INTO `change_groups` VALUES (10672,10238,17,'2026-07-21 14:07:49.000');
INSERT INTO `change_groups` VALUES (10673,10238,5,'2026-07-24 09:40:03.000');
INSERT INTO `change_groups` VALUES (10674,10238,11,'2026-08-04 13:38:48.000');
INSERT INTO `change_groups` VALUES (10675,10239,16,'2026-07-15 15:55:01.000');
INSERT INTO `change_groups` VALUES (10676,10240,4,'2026-07-14 17:01:16.000');
INSERT INTO `change_groups` VALUES (10677,10241,4,'2026-07-16 15:30:55.000');
INSERT INTO `change_groups` VALUES (10678,10241,5,'2026-07-21 06:25:33.000');
INSERT INTO `change_groups` VALUES (10679,10241,5,'2026-07-26 07:22:56.000');
INSERT INTO `change_groups` VALUES (10680,10241,19,'2026-07-29 15:33:53.000');
INSERT INTO `change_groups` VALUES (10681,10241,9,'2026-08-02 08:36:39.000');
INSERT INTO `change_groups` VALUES (10682,10241,11,'2026-08-10 09:18:35.000');
INSERT INTO `change_groups` VALUES (10683,10242,6,'2026-07-18 16:50:37.000');
INSERT INTO `change_groups` VALUES (10684,10242,6,'2026-07-19 07:37:47.000');
INSERT INTO `change_groups` VALUES (10685,10243,10,'2026-07-22 08:14:39.000');
INSERT INTO `change_groups` VALUES (10686,10244,13,'2026-07-16 15:16:51.000');
INSERT INTO `change_groups` VALUES (10687,10244,3,'2026-07-30 11:15:24.000');
INSERT INTO `change_groups` VALUES (10688,10245,2,'2026-07-29 13:39:08.000');
INSERT INTO `change_groups` VALUES (10689,10245,2,'2026-08-11 13:29:47.000');
INSERT INTO `change_groups` VALUES (10690,10246,15,'2026-07-24 11:13:53.000');
INSERT INTO `change_groups` VALUES (10691,10247,14,'2026-07-24 12:29:02.000');
INSERT INTO `change_groups` VALUES (10692,10247,14,'2026-07-29 06:26:16.000');
INSERT INTO `change_groups` VALUES (10693,10247,14,'2026-08-03 17:17:59.000');
INSERT INTO `change_groups` VALUES (10694,10248,23,'2026-08-02 16:53:52.000');
INSERT INTO `change_groups` VALUES (10695,10248,23,'2026-08-10 15:09:43.000');
INSERT INTO `change_groups` VALUES (10696,10248,23,'2026-08-12 14:41:26.000');
INSERT INTO `change_groups` VALUES (10697,10249,2,'2026-07-25 14:21:03.000');
INSERT INTO `change_groups` VALUES (10698,10249,17,'2026-07-30 08:08:22.000');
INSERT INTO `change_groups` VALUES (10699,10249,7,'2026-08-09 07:04:08.000');
INSERT INTO `change_groups` VALUES (10700,10249,6,'2026-08-18 14:00:09.000');
INSERT INTO `change_groups` VALUES (10701,10250,23,'2026-07-26 17:40:03.000');
INSERT INTO `change_groups` VALUES (10702,10251,15,'2026-07-28 17:14:48.000');
INSERT INTO `change_groups` VALUES (10703,10251,6,'2026-08-03 15:53:10.000');
INSERT INTO `change_groups` VALUES (10704,10251,11,'2026-08-04 08:14:07.000');
INSERT INTO `change_groups` VALUES (10705,10251,2,'2026-08-06 14:34:51.000');
INSERT INTO `change_groups` VALUES (10706,10251,2,'2026-08-14 08:55:39.000');
INSERT INTO `change_groups` VALUES (10707,10252,16,'2026-08-01 07:54:17.000');
INSERT INTO `change_groups` VALUES (10708,10252,16,'2026-08-01 10:03:58.000');
INSERT INTO `change_groups` VALUES (10709,10252,15,'2026-08-10 11:35:33.000');
INSERT INTO `change_groups` VALUES (10710,10252,5,'2026-08-22 06:24:51.000');
INSERT INTO `change_groups` VALUES (10711,10253,9,'2026-08-08 08:41:23.000');
INSERT INTO `change_groups` VALUES (10712,10253,5,'2026-08-10 15:14:26.000');
INSERT INTO `change_groups` VALUES (10713,10253,9,'2026-08-11 07:11:34.000');
INSERT INTO `change_groups` VALUES (10714,10254,16,'2026-08-11 14:30:42.000');
INSERT INTO `change_groups` VALUES (10715,10255,8,'2026-08-07 12:20:02.000');
INSERT INTO `change_groups` VALUES (10716,10255,8,'2026-08-07 17:15:23.000');
INSERT INTO `change_groups` VALUES (10717,10255,10,'2026-08-08 13:32:02.000');
INSERT INTO `change_groups` VALUES (10718,10255,14,'2026-08-16 17:32:32.000');
INSERT INTO `change_groups` VALUES (10719,10255,14,'2026-08-23 16:37:27.000');
INSERT INTO `change_groups` VALUES (10720,10256,5,'2026-08-11 08:40:17.000');
INSERT INTO `change_groups` VALUES (10721,10256,5,'2026-08-11 13:36:15.000');
INSERT INTO `change_groups` VALUES (10722,10256,5,'2026-08-16 11:38:49.000');
INSERT INTO `change_groups` VALUES (10723,10257,13,'2026-08-07 14:58:56.000');
INSERT INTO `change_groups` VALUES (10724,10257,10,'2026-08-10 13:54:50.000');
INSERT INTO `change_groups` VALUES (10725,10257,6,'2026-08-10 15:35:40.000');
INSERT INTO `change_groups` VALUES (10726,10257,10,'2026-08-17 11:19:57.000');
INSERT INTO `change_groups` VALUES (10727,10258,9,'2026-08-07 08:59:32.000');
INSERT INTO `change_groups` VALUES (10728,10258,22,'2026-08-08 12:09:43.000');
INSERT INTO `change_groups` VALUES (10729,10258,9,'2026-08-09 17:39:42.000');
INSERT INTO `change_groups` VALUES (10730,10258,23,'2026-08-13 07:50:13.000');
INSERT INTO `change_groups` VALUES (10731,10258,23,'2026-08-26 09:42:05.000');
INSERT INTO `change_groups` VALUES (10732,10259,11,'2026-08-14 14:18:57.000');
INSERT INTO `change_groups` VALUES (10733,10260,9,'2026-08-15 11:21:18.000');
INSERT INTO `change_groups` VALUES (10734,10260,21,'2026-08-16 16:20:28.000');
INSERT INTO `change_groups` VALUES (10735,10260,12,'2026-08-17 13:15:53.000');
INSERT INTO `change_groups` VALUES (10736,10260,22,'2026-08-19 17:17:09.000');
INSERT INTO `change_groups` VALUES (10737,10260,12,'2026-08-21 15:36:02.000');
INSERT INTO `change_groups` VALUES (10738,10260,12,'2026-08-28 15:53:04.000');
INSERT INTO `change_groups` VALUES (10739,10261,23,'2026-08-12 09:28:28.000');
INSERT INTO `change_groups` VALUES (10740,10261,22,'2026-08-19 11:45:59.000');
INSERT INTO `change_groups` VALUES (10741,10262,22,'2026-08-12 12:42:35.000');
INSERT INTO `change_groups` VALUES (10742,10262,22,'2026-08-15 10:24:16.000');
INSERT INTO `change_groups` VALUES (10743,10262,15,'2026-08-19 07:17:39.000');
INSERT INTO `change_groups` VALUES (10744,10262,11,'2026-08-30 06:08:38.000');
INSERT INTO `change_groups` VALUES (10745,10263,5,'2026-08-18 10:32:09.000');
INSERT INTO `change_groups` VALUES (10746,10263,6,'2026-09-12 09:08:43.000');
INSERT INTO `change_groups` VALUES (10747,10265,9,'2026-08-26 17:55:51.000');
INSERT INTO `change_groups` VALUES (10748,10266,16,'2026-08-22 10:20:50.000');
INSERT INTO `change_groups` VALUES (10749,10266,2,'2026-08-24 07:46:40.000');
INSERT INTO `change_groups` VALUES (10750,10266,3,'2026-09-05 16:12:07.000');
INSERT INTO `change_groups` VALUES (10751,10267,15,'2026-08-25 11:29:24.000');
INSERT INTO `change_groups` VALUES (10752,10267,21,'2026-09-03 13:18:23.000');
INSERT INTO `change_groups` VALUES (10753,10268,10,'2026-09-01 15:13:02.000');
INSERT INTO `change_groups` VALUES (10754,10268,8,'2026-09-04 15:56:25.000');
INSERT INTO `change_groups` VALUES (10755,10268,8,'2026-09-08 13:10:15.000');
INSERT INTO `change_groups` VALUES (10756,10269,13,'2026-08-31 06:42:38.000');
INSERT INTO `change_groups` VALUES (10757,10269,17,'2026-09-07 06:21:14.000');
INSERT INTO `change_groups` VALUES (10758,10270,10,'2026-08-31 07:23:42.000');
INSERT INTO `change_groups` VALUES (10759,10270,10,'2026-09-03 17:47:57.000');
INSERT INTO `change_groups` VALUES (10760,10270,10,'2026-09-14 14:25:19.000');
INSERT INTO `change_groups` VALUES (10761,10271,13,'2026-08-31 09:39:24.000');
INSERT INTO `change_groups` VALUES (10762,10271,13,'2026-09-06 09:31:57.000');
INSERT INTO `change_groups` VALUES (10763,10271,13,'2026-09-11 11:57:09.000');
INSERT INTO `change_groups` VALUES (10764,10271,2,'2026-09-16 14:22:14.000');
INSERT INTO `change_groups` VALUES (10765,10272,3,'2026-08-28 17:47:32.000');
INSERT INTO `change_groups` VALUES (10766,10272,15,'2026-08-29 07:00:21.000');
INSERT INTO `change_groups` VALUES (10767,10272,16,'2026-09-09 13:52:10.000');
INSERT INTO `change_groups` VALUES (10768,10273,3,'2026-08-31 13:13:09.000');
INSERT INTO `change_groups` VALUES (10769,10275,7,'2026-09-16 11:09:25.000');
INSERT INTO `change_groups` VALUES (10770,10276,5,'2026-09-06 10:11:16.000');
INSERT INTO `change_groups` VALUES (10771,10276,5,'2026-09-07 12:50:30.000');
INSERT INTO `change_groups` VALUES (10772,10276,6,'2026-09-08 06:27:02.000');
INSERT INTO `change_groups` VALUES (10773,10276,11,'2026-09-09 07:20:25.000');
INSERT INTO `change_groups` VALUES (10774,10277,21,'2026-09-01 17:36:57.000');
INSERT INTO `change_groups` VALUES (10775,10277,21,'2026-09-13 09:13:09.000');
INSERT INTO `change_groups` VALUES (10776,10277,11,'2026-09-18 07:07:36.000');
INSERT INTO `change_groups` VALUES (10777,10277,21,'2026-09-25 16:57:47.000');
INSERT INTO `change_groups` VALUES (10778,10277,11,'2026-09-28 14:17:14.000');
INSERT INTO `change_groups` VALUES (10779,10278,8,'2026-09-10 10:08:15.000');
INSERT INTO `change_groups` VALUES (10780,10278,8,'2026-09-20 16:09:55.000');
INSERT INTO `change_groups` VALUES (10781,10279,2,'2026-09-04 07:06:52.000');
INSERT INTO `change_groups` VALUES (10782,10279,2,'2026-09-10 09:32:33.000');
INSERT INTO `change_groups` VALUES (10783,10279,2,'2026-09-13 09:22:04.000');
INSERT INTO `change_groups` VALUES (10784,10279,5,'2026-09-20 16:00:45.000');
INSERT INTO `change_groups` VALUES (10785,10280,6,'2026-09-05 06:17:55.000');
INSERT INTO `change_groups` VALUES (10786,10280,17,'2026-09-07 09:24:47.000');
INSERT INTO `change_groups` VALUES (10787,10280,6,'2026-09-12 06:04:35.000');
INSERT INTO `change_groups` VALUES (10788,10280,17,'2026-09-12 13:38:40.000');
INSERT INTO `change_groups` VALUES (10789,10280,17,'2026-09-13 13:35:10.000');
INSERT INTO `change_groups` VALUES (10790,10280,6,'2026-09-14 14:13:34.000');
INSERT INTO `change_groups` VALUES (10791,10280,3,'2026-09-19 07:55:26.000');
INSERT INTO `change_groups` VALUES (10792,10281,15,'2026-09-11 08:59:35.000');
INSERT INTO `change_groups` VALUES (10793,10281,16,'2026-09-20 11:22:19.000');
INSERT INTO `change_groups` VALUES (10794,10281,3,'2026-09-22 16:30:31.000');
INSERT INTO `change_groups` VALUES (10795,10281,13,'2026-10-01 16:00:00.000');
INSERT INTO `change_groups` VALUES (10796,10282,2,'2026-09-10 14:30:22.000');
INSERT INTO `change_groups` VALUES (10797,10282,18,'2026-09-13 07:06:13.000');
INSERT INTO `change_groups` VALUES (10798,10282,18,'2026-09-15 08:33:09.000');
INSERT INTO `change_groups` VALUES (10799,10282,18,'2026-09-27 13:17:52.000');
INSERT INTO `change_groups` VALUES (10800,10282,2,'2026-10-01 16:00:00.000');
INSERT INTO `change_groups` VALUES (10801,10283,7,'2026-09-20 06:59:21.000');
INSERT INTO `change_groups` VALUES (10802,10284,15,'2026-09-22 06:30:31.000');
INSERT INTO `change_groups` VALUES (10803,10284,15,'2026-09-24 08:25:26.000');
INSERT INTO `change_groups` VALUES (10804,10284,15,'2026-09-25 09:48:18.000');
INSERT INTO `change_groups` VALUES (10805,10285,22,'2026-09-14 17:39:52.000');
INSERT INTO `change_groups` VALUES (10806,10285,11,'2026-09-26 14:53:37.000');
INSERT INTO `change_groups` VALUES (10807,10286,9,'2026-09-14 15:45:58.000');
INSERT INTO `change_groups` VALUES (10808,10286,15,'2026-09-19 09:07:05.000');
INSERT INTO `change_groups` VALUES (10809,10286,16,'2026-09-25 11:32:36.000');
INSERT INTO `change_groups` VALUES (10810,10286,16,'2026-09-29 06:05:08.000');
INSERT INTO `change_groups` VALUES (10811,10287,16,'2026-09-11 15:03:52.000');
INSERT INTO `change_groups` VALUES (10812,10287,2,'2026-09-21 14:59:40.000');
INSERT INTO `change_groups` VALUES (10813,10288,3,'2026-09-18 09:22:26.000');
INSERT INTO `change_groups` VALUES (10814,10288,5,'2026-09-20 10:46:06.000');
INSERT INTO `change_groups` VALUES (10815,10288,4,'2026-09-22 10:23:47.000');
INSERT INTO `change_groups` VALUES (10816,10288,9,'2026-09-24 11:26:42.000');
INSERT INTO `change_groups` VALUES (10817,10288,3,'2026-09-25 11:01:35.000');
INSERT INTO `change_groups` VALUES (10818,10289,16,'2026-09-17 12:20:37.000');
INSERT INTO `change_groups` VALUES (10819,10289,6,'2026-09-20 12:38:18.000');
INSERT INTO `change_groups` VALUES (10820,10289,4,'2026-09-27 16:36:41.000');
INSERT INTO `change_groups` VALUES (10821,10289,2,'2026-09-27 16:50:44.000');
INSERT INTO `change_groups` VALUES (10822,10290,17,'2026-09-17 12:27:58.000');
INSERT INTO `change_groups` VALUES (10823,10290,7,'2026-10-01 16:00:00.000');
INSERT INTO `change_groups` VALUES (10824,10291,4,'2026-09-25 07:04:55.000');
INSERT INTO `change_groups` VALUES (10825,10291,11,'2026-09-29 09:50:22.000');
INSERT INTO `change_groups` VALUES (10826,10291,4,'2026-10-01 16:00:00.000');
INSERT INTO `change_groups` VALUES (10827,10291,15,'2026-10-01 16:00:00.000');
INSERT INTO `change_groups` VALUES (10828,10291,4,'2026-10-01 16:00:01.000');
INSERT INTO `change_groups` VALUES (10829,10292,15,'2026-09-28 16:03:06.000');
INSERT INTO `change_groups` VALUES (10830,10293,11,'2026-09-28 11:02:18.000');
INSERT INTO `change_groups` VALUES (10831,10293,23,'2026-10-01 16:00:00.000');
INSERT INTO `change_groups` VALUES (10832,10293,21,'2026-10-01 16:00:01.000');
INSERT INTO `change_groups` VALUES (10833,10294,4,'2026-09-23 06:12:52.000');
INSERT INTO `change_groups` VALUES (10834,10294,12,'2026-09-26 10:57:57.000');
INSERT INTO `change_groups` VALUES (10835,10294,16,'2026-10-01 16:00:00.000');
INSERT INTO `change_groups` VALUES (10836,10294,17,'2026-10-01 16:00:01.000');
INSERT INTO `change_groups` VALUES (10837,10294,3,'2026-10-01 16:00:02.000');
INSERT INTO `change_groups` VALUES (10838,10295,6,'2026-09-25 10:23:14.000');
INSERT INTO `change_groups` VALUES (10839,10295,2,'2026-09-26 12:27:55.000');
INSERT INTO `change_groups` VALUES (10840,10295,2,'2026-09-29 09:30:45.000');
INSERT INTO `change_groups` VALUES (10841,10295,4,'2026-10-01 16:00:00.000');
INSERT INTO `change_groups` VALUES (10842,10295,11,'2026-10-01 16:00:00.000');
INSERT INTO `change_groups` VALUES (10843,10295,6,'2026-10-01 16:00:00.000');
INSERT INTO `change_groups` VALUES (10844,10295,6,'2026-10-01 16:00:01.000');
INSERT INTO `change_groups` VALUES (10845,10296,23,'2026-10-01 16:00:00.000');
INSERT INTO `change_groups` VALUES (10846,10297,13,'2026-10-01 14:37:53.000');
INSERT INTO `change_groups` VALUES (10847,10297,15,'2026-10-01 16:00:00.000');
INSERT INTO `change_groups` VALUES (10848,10297,16,'2026-10-01 16:00:00.000');
INSERT INTO `change_groups` VALUES (10849,10297,5,'2026-10-01 16:00:01.000');
INSERT INTO `change_groups` VALUES (10850,10298,6,'2026-09-30 15:13:03.000');
INSERT INTO `change_groups` VALUES (10851,10298,11,'2026-10-01 16:00:00.000');
INSERT INTO `change_groups` VALUES (10852,10298,17,'2026-10-01 16:00:00.000');
INSERT INTO `change_groups` VALUES (10853,10298,17,'2026-10-01 16:00:01.000');
INSERT INTO `change_groups` VALUES (10854,10298,11,'2026-10-01 16:00:02.000');
INSERT INTO `change_groups` VALUES (10855,10299,10,'2026-10-01 16:00:00.000');
INSERT INTO `change_groups` VALUES (10856,10299,1,'2026-10-01 16:00:00.000');
INSERT INTO `change_groups` VALUES (10857,10299,10,'2026-10-01 16:00:01.000');
/*!40000 ALTER TABLE `change_groups` ENABLE KEYS */;
DROP TABLE IF EXISTS `change_items`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `change_items` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `group_id` bigint(20) NOT NULL,
  `field_type` varchar(255) NOT NULL,
  `field` varchar(255) NOT NULL,
  `old_value` longtext DEFAULT NULL,
  `old_string` longtext DEFAULT NULL,
  `new_value` longtext DEFAULT NULL,
  `new_string` longtext DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_ci_group` (`group_id`),
  KEY `ix_ci_field` (`field`,`group_id`),
  CONSTRAINT `fk_ci_group` FOREIGN KEY (`group_id`) REFERENCES `change_groups` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=10993 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `change_items` DISABLE KEYS */;
INSERT INTO `change_items` VALUES (10000,10000,'jira','Fix Version',NULL,NULL,'10003','2.1');
INSERT INTO `change_items` VALUES (10001,10001,'jira','Link',NULL,NULL,'DEMO-2','This issue relates to DEMO-2');
INSERT INTO `change_items` VALUES (10002,10002,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10003,10003,'jira','Fix Version',NULL,NULL,'10002','2.0');
INSERT INTO `change_items` VALUES (10004,10004,'jira','status','3','In Progress','10001','Done');
INSERT INTO `change_items` VALUES (10005,10004,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10006,10005,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10007,10006,'jira','Attachment',NULL,NULL,'10002','spec.pdf');
INSERT INTO `change_items` VALUES (10008,10007,'jira','assignee','paul.martin','Paul Martin','JIRAUSER10000','Marie Durand');
INSERT INTO `change_items` VALUES (10009,10008,'jira','Attachment',NULL,NULL,'10001','capture.png');
INSERT INTO `change_items` VALUES (10010,10009,'jira','Attachment',NULL,NULL,'10000','logs-prod.txt');
INSERT INTO `change_items` VALUES (10011,10010,'jira','summary',NULL,'[À qualifier] Simplifier l\'export CSV des factures',NULL,'Simplifier l\'export CSV des factures');
INSERT INTO `change_items` VALUES (10012,10011,'jira','Link',NULL,NULL,'OPS-10','This issue relates to OPS-10');
INSERT INTO `change_items` VALUES (10013,10012,'jira','summary',NULL,'[À qualifier] Tester le tableau des commandes',NULL,'Tester le tableau des commandes');
INSERT INTO `change_items` VALUES (10014,10013,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10015,10014,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10016,10015,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10017,10015,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10018,10016,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10019,10017,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10020,10018,'jira','status','3','In Progress','10000','To Do');
INSERT INTO `change_items` VALUES (10021,10019,'jira','Link',NULL,NULL,'DEMO-28','This issue duplicates DEMO-28');
INSERT INTO `change_items` VALUES (10022,10020,'jira','priority','3','Medium','2','High');
INSERT INTO `change_items` VALUES (10023,10021,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10024,10022,'jira','status','3','In Progress','10001','Done');
INSERT INTO `change_items` VALUES (10025,10022,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10026,10023,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10027,10024,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10028,10025,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10029,10026,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10030,10026,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10031,10027,'jira','description',NULL,'La supervision a levé l\'alerte trois nuits de suite.',NULL,'h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n\n{code:sql}\nSELECT id, statut FROM commande\nWHERE cree_le > now() - interval \'7 days\';\n{code}');
INSERT INTO `change_items` VALUES (10032,10028,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10033,10029,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10034,10030,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10035,10030,'jira','resolution',NULL,NULL,'3','Duplicate');
INSERT INTO `change_items` VALUES (10036,10031,'jira','Link',NULL,NULL,'DEMO-57','This issue relates to DEMO-57');
INSERT INTO `change_items` VALUES (10037,10032,'jira','Attachment',NULL,NULL,'10004','capture.png');
INSERT INTO `change_items` VALUES (10038,10033,'jira','Attachment',NULL,NULL,'10003','export.csv');
INSERT INTO `change_items` VALUES (10039,10034,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10040,10035,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10041,10036,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10042,10036,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10043,10037,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10044,10037,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10045,10038,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10046,10039,'jira','status','3','In Progress','10000','To Do');
INSERT INTO `change_items` VALUES (10047,10040,'custom','Story Points',NULL,'8',NULL,'3');
INSERT INTO `change_items` VALUES (10048,10041,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10049,10042,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10050,10043,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10051,10043,'jira','resolution',NULL,NULL,'3','Duplicate');
INSERT INTO `change_items` VALUES (10052,10044,'jira','labels',NULL,'',NULL,'urgent perf');
INSERT INTO `change_items` VALUES (10053,10045,'jira','Attachment',NULL,NULL,'10007','maquette.jpg');
INSERT INTO `change_items` VALUES (10054,10046,'jira','summary',NULL,'[À qualifier] La recherche avancée ignore les caractères accentués',NULL,'La recherche avancée ignore les caractères accentués');
INSERT INTO `change_items` VALUES (10055,10047,'jira','Attachment',NULL,NULL,'10006','export.csv');
INSERT INTO `change_items` VALUES (10056,10048,'jira','Attachment',NULL,NULL,'10005','spec.pdf');
INSERT INTO `change_items` VALUES (10057,10049,'jira','Link',NULL,NULL,'WEB-47','This issue clones WEB-47');
INSERT INTO `change_items` VALUES (10058,10050,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10059,10050,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10060,10051,'jira','status','5','Resolved','4','Reopened');
INSERT INTO `change_items` VALUES (10061,10051,'jira','resolution','1','Fixed',NULL,NULL);
INSERT INTO `change_items` VALUES (10062,10052,'jira','priority','3','Medium','2','High');
INSERT INTO `change_items` VALUES (10063,10053,'jira','summary',NULL,'[À qualifier] Mettre à jour PostgreSQL sur srv-db-02',NULL,'Mettre à jour PostgreSQL sur srv-db-02');
INSERT INTO `change_items` VALUES (10064,10054,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10065,10055,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10066,10055,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10067,10056,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10068,10057,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10069,10058,'jira','assignee',NULL,NULL,'JIRAUSER10013','Maxime Bertrand');
INSERT INTO `change_items` VALUES (10070,10059,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10071,10059,'jira','resolution',NULL,NULL,'3','Duplicate');
INSERT INTO `change_items` VALUES (10072,10060,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10073,10061,'jira','description',NULL,'La supervision a levé l\'alerte trois nuits de suite.',NULL,'h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Les droits existants sont respectés\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}');
INSERT INTO `change_items` VALUES (10074,10062,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10075,10063,'jira','labels',NULL,'',NULL,'backend urgent');
INSERT INTO `change_items` VALUES (10076,10064,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10077,10065,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10078,10065,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10079,10066,'jira','status','5','Resolved','4','Reopened');
INSERT INTO `change_items` VALUES (10080,10066,'jira','resolution','1','Fixed',NULL,NULL);
INSERT INTO `change_items` VALUES (10081,10067,'jira','assignee',NULL,NULL,'JIRAUSER10018','Sophie Lambert');
INSERT INTO `change_items` VALUES (10082,10068,'jira','summary',NULL,'[À qualifier] Tester l\'export CSV des factures',NULL,'Tester l\'export CSV des factures');
INSERT INTO `change_items` VALUES (10083,10069,'jira','priority','1','Highest','3','Medium');
INSERT INTO `change_items` VALUES (10084,10070,'jira','Key',NULL,'OPS-5',NULL,'DEMO-121');
INSERT INTO `change_items` VALUES (10085,10070,'jira','project','10003','Exploitation','10000','Démo');
INSERT INTO `change_items` VALUES (10086,10071,'jira','assignee',NULL,NULL,'JIRAUSER10013','Maxime Bertrand');
INSERT INTO `change_items` VALUES (10087,10072,'jira','priority','1','Highest','3','Medium');
INSERT INTO `change_items` VALUES (10088,10073,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10089,10074,'jira','status','3','In Progress','10000','To Do');
INSERT INTO `change_items` VALUES (10090,10075,'jira','assignee',NULL,NULL,'JIRAUSER10011','Julien Roux');
INSERT INTO `change_items` VALUES (10091,10076,'jira','assignee',NULL,NULL,'JIRAUSER10005','Thomas Moreau');
INSERT INTO `change_items` VALUES (10092,10077,'jira','summary',NULL,'[À qualifier] La page de connexion renvoie une erreur 500',NULL,'La page de connexion renvoie une erreur 500');
INSERT INTO `change_items` VALUES (10093,10078,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10094,10079,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10095,10080,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10096,10080,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10097,10081,'jira','labels',NULL,'',NULL,'à-valider ux');
INSERT INTO `change_items` VALUES (10098,10082,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10099,10083,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10100,10084,'jira','labels',NULL,'',NULL,'sécurité frontend à-valider');
INSERT INTO `change_items` VALUES (10101,10085,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10102,10086,'jira','assignee','JIRAUSER10002','Camille Petit','JIRAUSER10007','Nicolas Laurent');
INSERT INTO `change_items` VALUES (10103,10087,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10104,10087,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10105,10088,'jira','Link',NULL,NULL,'DEMO-62','This issue blocks DEMO-62');
INSERT INTO `change_items` VALUES (10106,10089,'jira','priority','2','High','1','Highest');
INSERT INTO `change_items` VALUES (10107,10090,'jira','labels',NULL,'',NULL,'frontend à-valider client');
INSERT INTO `change_items` VALUES (10108,10091,'jira','labels',NULL,'',NULL,'dette-technique');
INSERT INTO `change_items` VALUES (10109,10092,'jira','assignee',NULL,NULL,'JIRAUSER10007','Nicolas Laurent');
INSERT INTO `change_items` VALUES (10110,10093,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10111,10094,'jira','status','3','In Progress','10000','To Do');
INSERT INTO `change_items` VALUES (10112,10095,'jira','Link',NULL,NULL,'DEMO-46','This issue blocks DEMO-46');
INSERT INTO `change_items` VALUES (10113,10096,'jira','Attachment',NULL,NULL,'10009','spec.pdf');
INSERT INTO `change_items` VALUES (10114,10097,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10115,10098,'jira','Attachment',NULL,NULL,'10008','capture.png');
INSERT INTO `change_items` VALUES (10116,10099,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10117,10100,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10118,10100,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10119,10101,'jira','Attachment',NULL,NULL,'10010','capture.png');
INSERT INTO `change_items` VALUES (10120,10102,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10121,10103,'jira','Attachment',NULL,NULL,'10011','maquette.jpg');
INSERT INTO `change_items` VALUES (10122,10104,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10123,10105,'jira','Attachment',NULL,NULL,'10012','export.csv');
INSERT INTO `change_items` VALUES (10124,10106,'jira','Link',NULL,NULL,'DEMO-31','This issue relates to DEMO-31');
INSERT INTO `change_items` VALUES (10125,10107,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10126,10108,'jira','priority','3','Medium','4','Low');
INSERT INTO `change_items` VALUES (10127,10109,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10128,10110,'jira','summary',NULL,'[À qualifier] Simplifier la gestion des rôles',NULL,'Simplifier la gestion des rôles');
INSERT INTO `change_items` VALUES (10129,10111,'jira','labels',NULL,'',NULL,'régression à-valider');
INSERT INTO `change_items` VALUES (10130,10112,'jira','assignee',NULL,NULL,'jdoe','Jeanne Dupont');
INSERT INTO `change_items` VALUES (10131,10113,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10132,10114,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10133,10115,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10134,10115,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10135,10116,'jira','Link',NULL,NULL,'OPS-24','This issue blocks OPS-24');
INSERT INTO `change_items` VALUES (10136,10117,'custom','Story Points',NULL,'2',NULL,'13');
INSERT INTO `change_items` VALUES (10137,10118,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10138,10119,'jira','Link',NULL,NULL,'DEMO-1','This issue blocks DEMO-1');
INSERT INTO `change_items` VALUES (10139,10120,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10140,10121,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10141,10121,'jira','resolution',NULL,NULL,'3','Duplicate');
INSERT INTO `change_items` VALUES (10142,10122,'jira','description',NULL,'Plusieurs clients nous ont remonté le problème depuis la dernière mise en production.',NULL,'h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur\n* Les tests de bout en bout couvrent le cas nominal\n\nLié à WEB-6.');
INSERT INTO `change_items` VALUES (10143,10123,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10144,10124,'jira','assignee','JIRAUSER10000','Marie Durand','jdoe','Jeanne Dupont');
INSERT INTO `change_items` VALUES (10145,10125,'jira','status','3','In Progress','10001','Done');
INSERT INTO `change_items` VALUES (10146,10125,'jira','resolution',NULL,NULL,'3','Duplicate');
INSERT INTO `change_items` VALUES (10147,10126,'jira','summary',NULL,'[À qualifier] Augmenter l\'espace disque sur srv-web-01',NULL,'Augmenter l\'espace disque sur srv-web-01');
INSERT INTO `change_items` VALUES (10148,10127,'jira','Attachment',NULL,NULL,'10013','spec.pdf');
INSERT INTO `change_items` VALUES (10149,10128,'jira','assignee',NULL,NULL,'JIRAUSER10003','Hugo Robert');
INSERT INTO `change_items` VALUES (10150,10129,'jira','Attachment',NULL,NULL,'10015','export.csv');
INSERT INTO `change_items` VALUES (10151,10130,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10152,10131,'jira','Attachment',NULL,NULL,'10014','capture.png');
INSERT INTO `change_items` VALUES (10153,10132,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10154,10133,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10155,10133,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10156,10134,'jira','Fix Version',NULL,NULL,'10001','1.1');
INSERT INTO `change_items` VALUES (10157,10135,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10158,10136,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10159,10137,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10160,10137,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10161,10138,'jira','Link',NULL,NULL,'DEMO-111','This issue clones DEMO-111');
INSERT INTO `change_items` VALUES (10162,10139,'jira','Attachment',NULL,NULL,'10017','spec.pdf');
INSERT INTO `change_items` VALUES (10163,10140,'jira','Attachment',NULL,NULL,'10018','export.csv');
INSERT INTO `change_items` VALUES (10164,10141,'custom','Story Points',NULL,'1',NULL,'5');
INSERT INTO `change_items` VALUES (10165,10142,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10166,10143,'jira','Attachment',NULL,NULL,'10016','capture.png');
INSERT INTO `change_items` VALUES (10167,10144,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10168,10145,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10169,10145,'jira','resolution',NULL,NULL,'5','Cannot Reproduce');
INSERT INTO `change_items` VALUES (10170,10146,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10171,10147,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10172,10148,'jira','assignee',NULL,NULL,'JIRAUSER10018','Sophie Lambert');
INSERT INTO `change_items` VALUES (10173,10149,'jira','Link',NULL,NULL,'DEMO-64','This issue duplicates DEMO-64');
INSERT INTO `change_items` VALUES (10174,10150,'jira','status','10000','To Do','10001','Done');
INSERT INTO `change_items` VALUES (10175,10150,'jira','resolution',NULL,NULL,'3','Duplicate');
INSERT INTO `change_items` VALUES (10176,10151,'custom','Story Points',NULL,'1',NULL,'5');
INSERT INTO `change_items` VALUES (10177,10152,'jira','status','10000','To Do','10001','Done');
INSERT INTO `change_items` VALUES (10178,10152,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10179,10153,'jira','priority','4','Low','3','Medium');
INSERT INTO `change_items` VALUES (10180,10154,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10181,10155,'jira','Attachment',NULL,NULL,'10019','logs-prod.txt');
INSERT INTO `change_items` VALUES (10182,10156,'jira','assignee',NULL,NULL,'JIRAUSER10011','Julien Roux');
INSERT INTO `change_items` VALUES (10183,10157,'jira','Fix Version',NULL,NULL,'10005','2025.2');
INSERT INTO `change_items` VALUES (10184,10158,'jira','Link',NULL,NULL,'DEMO-17','This issue blocks DEMO-17');
INSERT INTO `change_items` VALUES (10185,10159,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10186,10160,'jira','Attachment',NULL,NULL,'10021','maquette.jpg');
INSERT INTO `change_items` VALUES (10187,10161,'jira','Attachment',NULL,NULL,'10020','spec.pdf');
INSERT INTO `change_items` VALUES (10188,10162,'jira','assignee','JIRAUSER10002','Camille Petit','JIRAUSER10003','Hugo Robert');
INSERT INTO `change_items` VALUES (10189,10163,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10190,10164,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10191,10164,'jira','resolution',NULL,NULL,'4','Incomplete');
INSERT INTO `change_items` VALUES (10192,10165,'jira','labels',NULL,'',NULL,'client');
INSERT INTO `change_items` VALUES (10193,10166,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10194,10167,'jira','priority','5','Lowest','2','High');
INSERT INTO `change_items` VALUES (10195,10168,'jira','description',NULL,'Le besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.',NULL,'h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\n*Étapes pour reproduire :*\n# Ajouter deux articles au panier\n# Ouvrir le tableau de bord\n# Lancer l\'export\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\nVoir [la maquette|https://maquettes.exemple.fr/panier]. Lié à DEMO-23. [~lucas.bernard] peux-tu valider ?');
INSERT INTO `change_items` VALUES (10196,10169,'jira','summary',NULL,'[À qualifier] L\'API de facturation ne respecte pas les droits d\'accès',NULL,'L\'API de facturation ne respecte pas les droits d\'accès');
INSERT INTO `change_items` VALUES (10197,10170,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10198,10171,'custom','Story Points',NULL,'2',NULL,'3');
INSERT INTO `change_items` VALUES (10199,10172,'jira','description',NULL,'C\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.',NULL,'h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\n*Étapes pour reproduire :*\n# Ouvrir le tableau de bord\n# Rafraîchir la page\n# Ajouter deux articles au panier\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412].');
INSERT INTO `change_items` VALUES (10200,10173,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10201,10174,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10202,10175,'jira','status','10002','In Review','3','In Progress');
INSERT INTO `change_items` VALUES (10203,10176,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10204,10177,'jira','status','3','In Progress','10001','Done');
INSERT INTO `change_items` VALUES (10205,10177,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10206,10178,'jira','Link',NULL,NULL,'OPS-18','This issue relates to OPS-18');
INSERT INTO `change_items` VALUES (10207,10179,'jira','Attachment',NULL,NULL,'10023','export.csv');
INSERT INTO `change_items` VALUES (10208,10180,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10209,10181,'jira','Attachment',NULL,NULL,'10024','spec.pdf');
INSERT INTO `change_items` VALUES (10210,10182,'jira','description',NULL,'La supervision a levé l\'alerte trois nuits de suite.',NULL,'h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}\n\n!maquette.jpg!');
INSERT INTO `change_items` VALUES (10211,10183,'jira','priority','2','High','1','Highest');
INSERT INTO `change_items` VALUES (10212,10184,'jira','Attachment',NULL,NULL,'10022','maquette.jpg');
INSERT INTO `change_items` VALUES (10213,10185,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10214,10186,'jira','Link',NULL,NULL,'DEMO-41','This issue clones DEMO-41');
INSERT INTO `change_items` VALUES (10215,10187,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10216,10188,'jira','assignee','JIRAUSER10000','Marie Durand','jdoe','Jeanne Dupont');
INSERT INTO `change_items` VALUES (10217,10189,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10218,10190,'jira','Fix Version',NULL,NULL,'10005','2025.2');
INSERT INTO `change_items` VALUES (10219,10191,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10220,10192,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10221,10193,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10222,10193,'jira','resolution',NULL,NULL,'2','Won\'t Fix');
INSERT INTO `change_items` VALUES (10223,10194,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10224,10195,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10225,10195,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10226,10196,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10227,10197,'jira','status','1','Open','6','Closed');
INSERT INTO `change_items` VALUES (10228,10197,'jira','resolution',NULL,NULL,'2','Won\'t Fix');
INSERT INTO `change_items` VALUES (10229,10198,'jira','Link',NULL,NULL,'WEB-82','This issue relates to WEB-82');
INSERT INTO `change_items` VALUES (10230,10199,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10231,10199,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10232,10200,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10233,10201,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10234,10202,'jira','priority','5','Lowest','4','Low');
INSERT INTO `change_items` VALUES (10235,10203,'jira','Attachment',NULL,NULL,'10025','capture.png');
INSERT INTO `change_items` VALUES (10236,10204,'jira','status','3','In Progress','10000','To Do');
INSERT INTO `change_items` VALUES (10237,10205,'jira','Link',NULL,NULL,'WEB-37','This issue clones WEB-37');
INSERT INTO `change_items` VALUES (10238,10206,'custom','Story Points',NULL,'2',NULL,'5');
INSERT INTO `change_items` VALUES (10239,10207,'jira','summary',NULL,'[À qualifier] En tant que client, je veux recevoir une alerte de stock',NULL,'En tant que client, je veux recevoir une alerte de stock');
INSERT INTO `change_items` VALUES (10240,10208,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10241,10209,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10242,10210,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10243,10210,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10244,10211,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10245,10211,'jira','resolution',NULL,NULL,'2','Won\'t Fix');
INSERT INTO `change_items` VALUES (10246,10212,'jira','assignee',NULL,NULL,'JIRAUSER10007','Nicolas Laurent');
INSERT INTO `change_items` VALUES (10247,10213,'jira','priority','5','Lowest','3','Medium');
INSERT INTO `change_items` VALUES (10248,10214,'jira','assignee','JIRAUSER10010','Sarah Leroy','JIRAUSER10004','Léa Richard');
INSERT INTO `change_items` VALUES (10249,10215,'jira','summary',NULL,'[À qualifier] En tant que client, je veux réinitialiser mon mot de passe',NULL,'En tant que client, je veux réinitialiser mon mot de passe');
INSERT INTO `change_items` VALUES (10250,10216,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10251,10217,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10252,10218,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10253,10218,'jira','resolution',NULL,NULL,'2','Won\'t Fix');
INSERT INTO `change_items` VALUES (10254,10219,'jira','assignee','JIRAUSER10013','Maxime Bertrand','JIRAUSER10000','Marie Durand');
INSERT INTO `change_items` VALUES (10255,10220,'jira','Link',NULL,NULL,'OPS-17','This issue relates to OPS-17');
INSERT INTO `change_items` VALUES (10256,10221,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10257,10222,'jira','status','3','In Progress','10001','Done');
INSERT INTO `change_items` VALUES (10258,10222,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10259,10223,'jira','summary',NULL,'[À qualifier] En tant que agent du support, je veux suivre ma livraison',NULL,'En tant que agent du support, je veux suivre ma livraison');
INSERT INTO `change_items` VALUES (10260,10224,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10261,10225,'jira','labels',NULL,'',NULL,'frontend perf urgent');
INSERT INTO `change_items` VALUES (10262,10226,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10263,10227,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10264,10227,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10265,10228,'jira','assignee','JIRAUSER10013','Maxime Bertrand','JIRAUSER10000','Marie Durand');
INSERT INTO `change_items` VALUES (10266,10229,'jira','Attachment',NULL,NULL,'10028','spec.pdf');
INSERT INTO `change_items` VALUES (10267,10230,'jira','Attachment',NULL,NULL,'10027','logs-prod.txt');
INSERT INTO `change_items` VALUES (10268,10231,'jira','labels',NULL,'',NULL,'à-valider sécurité ux');
INSERT INTO `change_items` VALUES (10269,10232,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10270,10233,'jira','Attachment',NULL,NULL,'10026','capture.png');
INSERT INTO `change_items` VALUES (10271,10234,'jira','Attachment',NULL,NULL,'10029','spec.pdf');
INSERT INTO `change_items` VALUES (10272,10235,'jira','Attachment',NULL,NULL,'10030','capture.png');
INSERT INTO `change_items` VALUES (10273,10236,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10274,10237,'jira','Attachment',NULL,NULL,'10031','export.csv');
INSERT INTO `change_items` VALUES (10275,10238,'jira','Link',NULL,NULL,'DEMO-66','This issue relates to DEMO-66');
INSERT INTO `change_items` VALUES (10276,10239,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10277,10240,'jira','Attachment',NULL,NULL,'10032','logs-prod.txt');
INSERT INTO `change_items` VALUES (10278,10241,'jira','summary',NULL,'[À qualifier] Mettre à jour la grille salariale',NULL,'Mettre à jour la grille salariale');
INSERT INTO `change_items` VALUES (10279,10242,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10280,10243,'jira','Attachment',NULL,NULL,'10033','capture.png');
INSERT INTO `change_items` VALUES (10281,10244,'jira','Attachment',NULL,NULL,'10034','export.csv');
INSERT INTO `change_items` VALUES (10282,10245,'jira','Attachment',NULL,NULL,'10035','capture.png');
INSERT INTO `change_items` VALUES (10283,10246,'jira','Fix Version',NULL,NULL,'10002','2.0');
INSERT INTO `change_items` VALUES (10284,10247,'jira','Link',NULL,NULL,'DEMO-51','This issue blocks DEMO-51');
INSERT INTO `change_items` VALUES (10285,10248,'jira','Fix Version',NULL,NULL,'10004','2025.1');
INSERT INTO `change_items` VALUES (10286,10249,'jira','assignee',NULL,NULL,'JIRAUSER10013','Maxime Bertrand');
INSERT INTO `change_items` VALUES (10287,10250,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10288,10251,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10289,10251,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10290,10252,'jira','description',NULL,'Le comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.',NULL,'h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Les tests de bout en bout couvrent le cas nominal\n\n[~hugo.robert] peux-tu valider ?');
INSERT INTO `change_items` VALUES (10291,10253,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10292,10254,'jira','status','3','In Progress','10001','Done');
INSERT INTO `change_items` VALUES (10293,10254,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10294,10255,'jira','assignee',NULL,NULL,'JIRAUSER10000','Marie Durand');
INSERT INTO `change_items` VALUES (10295,10256,'jira','Attachment',NULL,NULL,'10036','export.csv');
INSERT INTO `change_items` VALUES (10296,10257,'jira','Attachment',NULL,NULL,'10038','capture.png');
INSERT INTO `change_items` VALUES (10297,10258,'jira','Attachment',NULL,NULL,'10037','logs-prod.txt');
INSERT INTO `change_items` VALUES (10298,10259,'jira','Link',NULL,NULL,'DEMO-2','This issue relates to DEMO-2');
INSERT INTO `change_items` VALUES (10299,10260,'jira','Attachment',NULL,NULL,'10039','maquette.jpg');
INSERT INTO `change_items` VALUES (10300,10261,'jira','Attachment',NULL,NULL,'10040','capture.png');
INSERT INTO `change_items` VALUES (10301,10262,'jira','assignee',NULL,NULL,'paul.martin','Paul Martin');
INSERT INTO `change_items` VALUES (10302,10263,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10303,10263,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10304,10264,'jira','Attachment',NULL,NULL,'10042','export.csv');
INSERT INTO `change_items` VALUES (10305,10265,'jira','Attachment',NULL,NULL,'10043','spec.pdf');
INSERT INTO `change_items` VALUES (10306,10266,'jira','Attachment',NULL,NULL,'10041','capture.png');
INSERT INTO `change_items` VALUES (10307,10267,'jira','assignee',NULL,NULL,'JIRAUSER10007','Nicolas Laurent');
INSERT INTO `change_items` VALUES (10308,10268,'jira','Fix Version',NULL,NULL,'10003','2.1');
INSERT INTO `change_items` VALUES (10309,10269,'jira','priority','4','Low','2','High');
INSERT INTO `change_items` VALUES (10310,10270,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10311,10271,'jira','Link',NULL,NULL,'DEMO-110','This issue relates to DEMO-110');
INSERT INTO `change_items` VALUES (10312,10272,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10313,10273,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10314,10274,'jira','Attachment',NULL,NULL,'10044','export.csv');
INSERT INTO `change_items` VALUES (10315,10275,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10316,10275,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10317,10276,'custom','Story Points',NULL,'0.5',NULL,'2');
INSERT INTO `change_items` VALUES (10318,10277,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10319,10278,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10320,10279,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10321,10279,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10322,10280,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10323,10281,'jira','description',NULL,'Plusieurs clients nous ont remonté le problème depuis la dernière mise en production.',NULL,'h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde');
INSERT INTO `change_items` VALUES (10324,10282,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10325,10283,'jira','status','3','In Progress','10000','To Do');
INSERT INTO `change_items` VALUES (10326,10284,'custom','Story Points',NULL,'13',NULL,'5');
INSERT INTO `change_items` VALUES (10327,10285,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10328,10286,'jira','priority','2','High','4','Low');
INSERT INTO `change_items` VALUES (10329,10287,'jira','assignee',NULL,NULL,'JIRAUSER10001','Lucas Bernard');
INSERT INTO `change_items` VALUES (10330,10288,'jira','Link',NULL,NULL,'DEMO-44','This issue duplicates DEMO-44');
INSERT INTO `change_items` VALUES (10331,10289,'custom','Story Points',NULL,'2',NULL,'3');
INSERT INTO `change_items` VALUES (10332,10290,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10333,10291,'jira','status','3','In Progress','10000','To Do');
INSERT INTO `change_items` VALUES (10334,10292,'jira','summary',NULL,'[À qualifier] Préparer l\'entretien annuel de M. Roux',NULL,'Préparer l\'entretien annuel de M. Roux');
INSERT INTO `change_items` VALUES (10335,10293,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10336,10294,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10337,10294,'jira','resolution',NULL,NULL,'5','Cannot Reproduce');
INSERT INTO `change_items` VALUES (10338,10295,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10339,10295,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10340,10296,'jira','description',NULL,'La supervision a levé l\'alerte trois nuits de suite.',NULL,'h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412].');
INSERT INTO `change_items` VALUES (10341,10297,'jira','labels',NULL,'',NULL,'sécurité');
INSERT INTO `change_items` VALUES (10342,10298,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10343,10299,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10344,10299,'jira','resolution',NULL,NULL,'4','Incomplete');
INSERT INTO `change_items` VALUES (10345,10300,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10346,10301,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10347,10301,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10348,10302,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10349,10303,'jira','status','3','In Progress','10001','Done');
INSERT INTO `change_items` VALUES (10350,10303,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10351,10304,'jira','description',NULL,'La supervision a levé l\'alerte trois nuits de suite.',NULL,'h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Un message clair s\'affiche en cas d\'erreur\n* Les droits existants sont respectés\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}\n\n[~julien.roux] peux-tu valider ?');
INSERT INTO `change_items` VALUES (10352,10305,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10353,10306,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10354,10307,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10355,10307,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10356,10308,'jira','Attachment',NULL,NULL,'10045','logs-prod.txt');
INSERT INTO `change_items` VALUES (10357,10309,'jira','Attachment',NULL,NULL,'10046','capture.png');
INSERT INTO `change_items` VALUES (10358,10310,'custom','Story Points',NULL,'2',NULL,'1');
INSERT INTO `change_items` VALUES (10359,10311,'jira','assignee','JIRAUSER10011','Julien Roux','JIRAUSER10012','Inès David');
INSERT INTO `change_items` VALUES (10360,10312,'jira','Link',NULL,NULL,'DEMO-88','This issue relates to DEMO-88');
INSERT INTO `change_items` VALUES (10361,10313,'jira','description',NULL,'La supervision a levé l\'alerte trois nuits de suite.',NULL,'h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412].');
INSERT INTO `change_items` VALUES (10362,10314,'jira','assignee','JIRAUSER10003','Hugo Robert','JIRAUSER10007','Nicolas Laurent');
INSERT INTO `change_items` VALUES (10363,10315,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10364,10316,'jira','Link',NULL,NULL,'DEMO-75','This issue relates to DEMO-75');
INSERT INTO `change_items` VALUES (10365,10317,'jira','summary',NULL,'[À qualifier] Rédiger les tests unitaires',NULL,'Rédiger les tests unitaires');
INSERT INTO `change_items` VALUES (10366,10318,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10367,10319,'jira','status','3','In Progress','10000','To Do');
INSERT INTO `change_items` VALUES (10368,10320,'jira','summary',NULL,'[À qualifier] En tant que client, je veux partager un panier',NULL,'En tant que client, je veux partager un panier');
INSERT INTO `change_items` VALUES (10369,10321,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10370,10322,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10371,10323,'jira','assignee',NULL,NULL,'JIRAUSER10007','Nicolas Laurent');
INSERT INTO `change_items` VALUES (10372,10324,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10373,10325,'jira','status','10000','To Do','10001','Done');
INSERT INTO `change_items` VALUES (10374,10325,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10375,10326,'jira','assignee','JIRAUSER10000','Marie Durand','JIRAUSER10001','Lucas Bernard');
INSERT INTO `change_items` VALUES (10376,10327,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10377,10327,'jira','resolution',NULL,NULL,'3','Duplicate');
INSERT INTO `change_items` VALUES (10378,10328,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10379,10329,'jira','Attachment',NULL,NULL,'10047','logs-prod.txt');
INSERT INTO `change_items` VALUES (10380,10330,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10381,10331,'jira','Attachment',NULL,NULL,'10049','spec.pdf');
INSERT INTO `change_items` VALUES (10382,10332,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10383,10332,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10384,10333,'jira','Attachment',NULL,NULL,'10048','export.csv');
INSERT INTO `change_items` VALUES (10385,10334,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10386,10335,'jira','priority','5','Lowest','3','Medium');
INSERT INTO `change_items` VALUES (10387,10336,'jira','summary',NULL,'[À qualifier] Mettre à jour PostgreSQL sur srv-web-01',NULL,'Mettre à jour PostgreSQL sur srv-web-01');
INSERT INTO `change_items` VALUES (10388,10337,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10389,10338,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10390,10338,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10391,10339,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10392,10340,'jira','assignee','JIRAUSER10002','Camille Petit','JIRAUSER10009','Antoine Lefebvre');
INSERT INTO `change_items` VALUES (10393,10341,'jira','Link',NULL,NULL,'DEMO-92','This issue relates to DEMO-92');
INSERT INTO `change_items` VALUES (10394,10342,'jira','assignee',NULL,NULL,'JIRAUSER10003','Hugo Robert');
INSERT INTO `change_items` VALUES (10395,10343,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10396,10344,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10397,10345,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10398,10345,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10399,10346,'jira','assignee',NULL,NULL,'JIRAUSER10011','Julien Roux');
INSERT INTO `change_items` VALUES (10400,10347,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10401,10348,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10402,10349,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10403,10349,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10404,10350,'jira','priority','2','High','3','Medium');
INSERT INTO `change_items` VALUES (10405,10351,'jira','status','1','Open','6','Closed');
INSERT INTO `change_items` VALUES (10406,10351,'jira','resolution',NULL,NULL,'5','Cannot Reproduce');
INSERT INTO `change_items` VALUES (10407,10352,'jira','assignee',NULL,NULL,'JIRAUSER10003','Hugo Robert');
INSERT INTO `change_items` VALUES (10408,10353,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10409,10354,'jira','status','3','In Progress','10000','To Do');
INSERT INTO `change_items` VALUES (10410,10355,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10411,10356,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10412,10356,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10413,10357,'jira','assignee','JIRAUSER10005','Thomas Moreau','JIRAUSER10011','Julien Roux');
INSERT INTO `change_items` VALUES (10414,10358,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10415,10359,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10416,10359,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10417,10360,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10418,10361,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10419,10362,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10420,10363,'jira','status','3','In Progress','10001','Done');
INSERT INTO `change_items` VALUES (10421,10363,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10422,10364,'custom','Story Points',NULL,'13',NULL,'8');
INSERT INTO `change_items` VALUES (10423,10365,'jira','labels',NULL,'',NULL,'régression sécurité perf');
INSERT INTO `change_items` VALUES (10424,10366,'custom','Story Points',NULL,'13',NULL,'0.5');
INSERT INTO `change_items` VALUES (10425,10367,'jira','assignee','JIRAUSER10002','Camille Petit','paul.martin','Paul Martin');
INSERT INTO `change_items` VALUES (10426,10368,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10427,10369,'jira','priority','1','Highest','3','Medium');
INSERT INTO `change_items` VALUES (10428,10370,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10429,10371,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10430,10372,'jira','status','10002','In Review','3','In Progress');
INSERT INTO `change_items` VALUES (10431,10373,'jira','priority','1','Highest','3','Medium');
INSERT INTO `change_items` VALUES (10432,10374,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10433,10375,'jira','Attachment',NULL,NULL,'10051','export.csv');
INSERT INTO `change_items` VALUES (10434,10376,'jira','Attachment',NULL,NULL,'10050','capture.png');
INSERT INTO `change_items` VALUES (10435,10377,'jira','Attachment',NULL,NULL,'10052','maquette.jpg');
INSERT INTO `change_items` VALUES (10436,10378,'jira','assignee','JIRAUSER10012','Inès David','JIRAUSER10009','Antoine Lefebvre');
INSERT INTO `change_items` VALUES (10437,10379,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10438,10380,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10439,10380,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10440,10381,'jira','status','10000','To Do','10001','Done');
INSERT INTO `change_items` VALUES (10441,10381,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10442,10382,'jira','summary',NULL,'[À qualifier] En tant que client, je veux exporter mes factures',NULL,'En tant que client, je veux exporter mes factures');
INSERT INTO `change_items` VALUES (10443,10383,'jira','assignee',NULL,NULL,'JIRAUSER10012','Inès David');
INSERT INTO `change_items` VALUES (10444,10384,'jira','summary',NULL,'[À qualifier] En tant que agent du support, je veux filtrer les commandes par statut',NULL,'En tant que agent du support, je veux filtrer les commandes par statut');
INSERT INTO `change_items` VALUES (10445,10385,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10446,10386,'jira','status','3','In Progress','10000','To Do');
INSERT INTO `change_items` VALUES (10447,10387,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10448,10388,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10449,10389,'jira','status','10002','In Review','3','In Progress');
INSERT INTO `change_items` VALUES (10450,10390,'jira','Fix Version',NULL,NULL,'10003','2.1');
INSERT INTO `change_items` VALUES (10451,10391,'custom','Story Points',NULL,'1',NULL,'0.5');
INSERT INTO `change_items` VALUES (10452,10392,'jira','labels',NULL,'',NULL,'mobile sécurité perf');
INSERT INTO `change_items` VALUES (10453,10393,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10454,10394,'jira','status','3','In Progress','10000','To Do');
INSERT INTO `change_items` VALUES (10455,10395,'jira','Link',NULL,NULL,'DEMO-105','This issue clones DEMO-105');
INSERT INTO `change_items` VALUES (10456,10396,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10457,10397,'jira','assignee','JIRAUSER10009','Antoine Lefebvre','JIRAUSER10012','Inès David');
INSERT INTO `change_items` VALUES (10458,10398,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10459,10399,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10460,10399,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10461,10400,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10462,10401,'jira','labels',NULL,'',NULL,'backend');
INSERT INTO `change_items` VALUES (10463,10402,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10464,10403,'jira','Fix Version',NULL,NULL,'10004','2025.1');
INSERT INTO `change_items` VALUES (10465,10404,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10466,10405,'jira','status','10002','In Review','3','In Progress');
INSERT INTO `change_items` VALUES (10467,10406,'jira','Link',NULL,NULL,'WEB-57','This issue blocks WEB-57');
INSERT INTO `change_items` VALUES (10468,10407,'jira','Link',NULL,NULL,'DEMO-82','This issue relates to DEMO-82');
INSERT INTO `change_items` VALUES (10469,10408,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10470,10408,'jira','resolution',NULL,NULL,'2','Won\'t Fix');
INSERT INTO `change_items` VALUES (10471,10409,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10472,10410,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10473,10410,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10474,10411,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10475,10412,'jira','priority','2','High','4','Low');
INSERT INTO `change_items` VALUES (10476,10413,'jira','summary',NULL,'[À qualifier] Performance des pages produit',NULL,'Performance des pages produit');
INSERT INTO `change_items` VALUES (10477,10414,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10478,10415,'jira','status','3','In Progress','10000','To Do');
INSERT INTO `change_items` VALUES (10479,10416,'jira','assignee','jdoe','Jeanne Dupont','JIRAUSER10002','Camille Petit');
INSERT INTO `change_items` VALUES (10480,10417,'custom','Story Points',NULL,'8',NULL,'3');
INSERT INTO `change_items` VALUES (10481,10418,'jira','priority','3','Medium','5','Lowest');
INSERT INTO `change_items` VALUES (10482,10419,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10483,10420,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10484,10421,'jira','Attachment',NULL,NULL,'10053','maquette.jpg');
INSERT INTO `change_items` VALUES (10485,10422,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10486,10422,'jira','resolution',NULL,NULL,'2','Won\'t Fix');
INSERT INTO `change_items` VALUES (10487,10423,'jira','Attachment',NULL,NULL,'10054','spec.pdf');
INSERT INTO `change_items` VALUES (10488,10424,'jira','Attachment',NULL,NULL,'10055','export.csv');
INSERT INTO `change_items` VALUES (10489,10425,'jira','assignee','JIRAUSER10012','Inès David','JIRAUSER10013','Maxime Bertrand');
INSERT INTO `change_items` VALUES (10490,10426,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10491,10427,'jira','assignee',NULL,NULL,'JIRAUSER10007','Nicolas Laurent');
INSERT INTO `change_items` VALUES (10492,10428,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10493,10428,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10494,10429,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10495,10430,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10496,10431,'jira','status','3','In Progress','10001','Done');
INSERT INTO `change_items` VALUES (10497,10431,'jira','resolution',NULL,NULL,'2','Won\'t Fix');
INSERT INTO `change_items` VALUES (10498,10432,'custom','Story Points',NULL,'2',NULL,'0.5');
INSERT INTO `change_items` VALUES (10499,10433,'jira','assignee',NULL,NULL,'JIRAUSER10003','Hugo Robert');
INSERT INTO `change_items` VALUES (10500,10434,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10501,10435,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10502,10435,'jira','resolution',NULL,NULL,'5','Cannot Reproduce');
INSERT INTO `change_items` VALUES (10503,10436,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10504,10437,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10505,10437,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10506,10438,'jira','assignee',NULL,NULL,'JIRAUSER10010','Sarah Leroy');
INSERT INTO `change_items` VALUES (10507,10439,'jira','summary',NULL,'[À qualifier] Tester l\'export CSV des factures',NULL,'Tester l\'export CSV des factures');
INSERT INTO `change_items` VALUES (10508,10440,'jira','Attachment',NULL,NULL,'10056','maquette.jpg');
INSERT INTO `change_items` VALUES (10509,10441,'jira','description',NULL,'La supervision a levé l\'alerte trois nuits de suite.',NULL,'h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412]. Lié à WEB-36.');
INSERT INTO `change_items` VALUES (10510,10442,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10511,10443,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10512,10444,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10513,10444,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10514,10445,'jira','Fix Version',NULL,NULL,'10003','2.1');
INSERT INTO `change_items` VALUES (10515,10446,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10516,10447,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10517,10448,'jira','status','10002','In Review','3','In Progress');
INSERT INTO `change_items` VALUES (10518,10449,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10519,10450,'jira','labels',NULL,'',NULL,'régression urgent');
INSERT INTO `change_items` VALUES (10520,10451,'jira','Attachment',NULL,NULL,'10057','logs-prod.txt');
INSERT INTO `change_items` VALUES (10521,10452,'jira','priority','1','Highest','3','Medium');
INSERT INTO `change_items` VALUES (10522,10453,'jira','assignee',NULL,NULL,'JIRAUSER10002','Camille Petit');
INSERT INTO `change_items` VALUES (10523,10454,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10524,10455,'jira','status','3','In Progress','10000','To Do');
INSERT INTO `change_items` VALUES (10525,10456,'jira','assignee','JIRAUSER10010','Sarah Leroy','JIRAUSER10004','Léa Richard');
INSERT INTO `change_items` VALUES (10526,10457,'jira','priority','1','Highest','2','High');
INSERT INTO `change_items` VALUES (10527,10458,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10528,10458,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10529,10459,'jira','Attachment',NULL,NULL,'10058','logs-prod.txt');
INSERT INTO `change_items` VALUES (10530,10460,'jira','assignee',NULL,NULL,'paul.martin','Paul Martin');
INSERT INTO `change_items` VALUES (10531,10461,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10532,10462,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10533,10463,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10534,10463,'jira','resolution',NULL,NULL,'2','Won\'t Fix');
INSERT INTO `change_items` VALUES (10535,10464,'custom','Story Points',NULL,'3',NULL,'2');
INSERT INTO `change_items` VALUES (10536,10465,'jira','description',NULL,'C\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.',NULL,'h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\n*Étapes pour reproduire :*\n# Ouvrir le tableau de bord\n# Changer la langue en anglais\n# Lancer l\'export\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\nLié à WEB-14.');
INSERT INTO `change_items` VALUES (10537,10466,'jira','assignee','jdoe','Jeanne Dupont','JIRAUSER10013','Maxime Bertrand');
INSERT INTO `change_items` VALUES (10538,10467,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10539,10468,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10540,10468,'jira','resolution',NULL,NULL,'2','Won\'t Fix');
INSERT INTO `change_items` VALUES (10541,10469,'jira','Link',NULL,NULL,'WEB-42','This issue clones WEB-42');
INSERT INTO `change_items` VALUES (10542,10470,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10543,10471,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10544,10472,'jira','description',NULL,'Plusieurs clients nous ont remonté le problème depuis la dernière mise en production.',NULL,'h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}');
INSERT INTO `change_items` VALUES (10545,10473,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10546,10473,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10547,10474,'jira','Attachment',NULL,NULL,'10059','logs-prod.txt');
INSERT INTO `change_items` VALUES (10548,10475,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10549,10476,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10550,10477,'jira','description',NULL,'Le comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.',NULL,'h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n* Un message clair s\'affiche en cas d\'erreur\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412]. [~ines.david] peux-tu valider ?');
INSERT INTO `change_items` VALUES (10551,10478,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10552,10479,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10553,10480,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10554,10480,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10555,10481,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10556,10481,'jira','resolution',NULL,NULL,'4','Incomplete');
INSERT INTO `change_items` VALUES (10557,10482,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10558,10483,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10559,10484,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10560,10485,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10561,10485,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10562,10486,'jira','priority','2','High','3','Medium');
INSERT INTO `change_items` VALUES (10563,10487,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10564,10488,'jira','assignee',NULL,NULL,'JIRAUSER10009','Antoine Lefebvre');
INSERT INTO `change_items` VALUES (10565,10489,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10566,10490,'jira','Attachment',NULL,NULL,'10061','logs-prod.txt');
INSERT INTO `change_items` VALUES (10567,10491,'jira','Attachment',NULL,NULL,'10060','maquette.jpg');
INSERT INTO `change_items` VALUES (10568,10492,'jira','summary',NULL,'[À qualifier] Le calendrier des livraisons s\'affiche mal sur mobile',NULL,'Le calendrier des livraisons s\'affiche mal sur mobile');
INSERT INTO `change_items` VALUES (10569,10493,'jira','Attachment',NULL,NULL,'10063','maquette.jpg');
INSERT INTO `change_items` VALUES (10570,10494,'jira','Attachment',NULL,NULL,'10062','export.csv');
INSERT INTO `change_items` VALUES (10571,10495,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10572,10496,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10573,10496,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10574,10497,'jira','Attachment',NULL,NULL,'10064','capture.png');
INSERT INTO `change_items` VALUES (10575,10498,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10576,10499,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10577,10500,'jira','summary',NULL,'[À qualifier] Ouvrir le flux réseau entre srv-batch-01 et le VPN',NULL,'Ouvrir le flux réseau entre srv-batch-01 et le VPN');
INSERT INTO `change_items` VALUES (10578,10501,'jira','priority','5','Lowest','3','Medium');
INSERT INTO `change_items` VALUES (10579,10502,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10580,10503,'jira','assignee',NULL,NULL,'jdoe','Jeanne Dupont');
INSERT INTO `change_items` VALUES (10581,10504,'jira','Link',NULL,NULL,'DEMO-14','This issue blocks DEMO-14');
INSERT INTO `change_items` VALUES (10582,10505,'jira','priority','5','Lowest','3','Medium');
INSERT INTO `change_items` VALUES (10583,10506,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10584,10507,'jira','status','3','In Progress','10001','Done');
INSERT INTO `change_items` VALUES (10585,10507,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10586,10508,'jira','Link',NULL,NULL,'DEMO-63','This issue relates to DEMO-63');
INSERT INTO `change_items` VALUES (10587,10509,'jira','description',NULL,'La supervision a levé l\'alerte trois nuits de suite.',NULL,'h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur');
INSERT INTO `change_items` VALUES (10588,10510,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10589,10511,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10590,10511,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10591,10512,'jira','Attachment',NULL,NULL,'10066','export.csv');
INSERT INTO `change_items` VALUES (10592,10513,'jira','Attachment',NULL,NULL,'10065','capture.png');
INSERT INTO `change_items` VALUES (10593,10514,'jira','status','5','Resolved','4','Reopened');
INSERT INTO `change_items` VALUES (10594,10514,'jira','resolution','1','Fixed',NULL,NULL);
INSERT INTO `change_items` VALUES (10595,10515,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10596,10516,'jira','assignee','JIRAUSER10009','Antoine Lefebvre','JIRAUSER10012','Inès David');
INSERT INTO `change_items` VALUES (10597,10517,'jira','Attachment',NULL,NULL,'10068','spec.pdf');
INSERT INTO `change_items` VALUES (10598,10518,'jira','Fix Version',NULL,NULL,'10005','2025.2');
INSERT INTO `change_items` VALUES (10599,10519,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10600,10520,'jira','Attachment',NULL,NULL,'10067','capture.png');
INSERT INTO `change_items` VALUES (10601,10521,'jira','assignee','JIRAUSER10012','Inès David','JIRAUSER10003','Hugo Robert');
INSERT INTO `change_items` VALUES (10602,10522,'jira','summary',NULL,'[À qualifier] Le formulaire d\'inscription perd les données saisies',NULL,'Le formulaire d\'inscription perd les données saisies');
INSERT INTO `change_items` VALUES (10603,10523,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10604,10524,'jira','status','3','In Progress','10000','To Do');
INSERT INTO `change_items` VALUES (10605,10525,'jira','Link',NULL,NULL,'WEB-84','This issue blocks WEB-84');
INSERT INTO `change_items` VALUES (10606,10526,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10607,10527,'jira','summary',NULL,'[À qualifier] Corriger les retours de recette',NULL,'Corriger les retours de recette');
INSERT INTO `change_items` VALUES (10608,10528,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10609,10529,'jira','Fix Version',NULL,NULL,'10006','2026.1');
INSERT INTO `change_items` VALUES (10610,10530,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10611,10530,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10612,10531,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10613,10532,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10614,10533,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10615,10533,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10616,10534,'jira','summary',NULL,'[À qualifier] Superviser la file de messages sur proxy-01',NULL,'Superviser la file de messages sur proxy-01');
INSERT INTO `change_items` VALUES (10617,10535,'jira','Attachment',NULL,NULL,'10069','spec.pdf');
INSERT INTO `change_items` VALUES (10618,10536,'jira','Link',NULL,NULL,'WEB-50','This issue blocks WEB-50');
INSERT INTO `change_items` VALUES (10619,10537,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10620,10538,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10621,10539,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10622,10539,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10623,10540,'jira','summary',NULL,'[À qualifier] Préparer la recette',NULL,'Préparer la recette');
INSERT INTO `change_items` VALUES (10624,10541,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10625,10542,'jira','Fix Version',NULL,NULL,'10001','1.1');
INSERT INTO `change_items` VALUES (10626,10543,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10627,10544,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10628,10544,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10629,10545,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10630,10546,'jira','assignee','JIRAUSER10007','Nicolas Laurent','JIRAUSER10001','Lucas Bernard');
INSERT INTO `change_items` VALUES (10631,10547,'jira','priority','3','Medium','2','High');
INSERT INTO `change_items` VALUES (10632,10548,'jira','labels',NULL,'',NULL,'sécurité ux');
INSERT INTO `change_items` VALUES (10633,10549,'jira','Link',NULL,NULL,'WEB-6','This issue duplicates WEB-6');
INSERT INTO `change_items` VALUES (10634,10550,'jira','Fix Version',NULL,NULL,'10003','2.1');
INSERT INTO `change_items` VALUES (10635,10551,'jira','priority','4','Low','3','Medium');
INSERT INTO `change_items` VALUES (10636,10552,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10637,10553,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10638,10554,'jira','Attachment',NULL,NULL,'10070','maquette.jpg');
INSERT INTO `change_items` VALUES (10639,10555,'jira','summary',NULL,'[À qualifier] En tant que commercial, je veux recevoir une alerte de stock',NULL,'En tant que commercial, je veux recevoir une alerte de stock');
INSERT INTO `change_items` VALUES (10640,10556,'jira','Attachment',NULL,NULL,'10071','logs-prod.txt');
INSERT INTO `change_items` VALUES (10641,10557,'jira','Attachment',NULL,NULL,'10072','spec.pdf');
INSERT INTO `change_items` VALUES (10642,10558,'jira','Fix Version',NULL,NULL,'10005','2025.2');
INSERT INTO `change_items` VALUES (10643,10559,'jira','assignee',NULL,NULL,'JIRAUSER10002','Camille Petit');
INSERT INTO `change_items` VALUES (10644,10560,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10645,10561,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10646,10562,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10647,10562,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10648,10563,'custom','Story Points',NULL,'8',NULL,'2');
INSERT INTO `change_items` VALUES (10649,10564,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10650,10565,'jira','assignee',NULL,NULL,'paul.martin','Paul Martin');
INSERT INTO `change_items` VALUES (10651,10566,'jira','Link',NULL,NULL,'DEMO-25','This issue blocks DEMO-25');
INSERT INTO `change_items` VALUES (10652,10567,'custom','Story Points',NULL,'3',NULL,'5');
INSERT INTO `change_items` VALUES (10653,10568,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10654,10569,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10655,10570,'jira','assignee',NULL,NULL,'JIRAUSER10000','Marie Durand');
INSERT INTO `change_items` VALUES (10656,10571,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10657,10572,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10658,10572,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10659,10573,'custom','Story Points',NULL,'5',NULL,'13');
INSERT INTO `change_items` VALUES (10660,10574,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10661,10575,'jira','status','3','In Progress','10000','To Do');
INSERT INTO `change_items` VALUES (10662,10576,'jira','assignee',NULL,NULL,'JIRAUSER10005','Thomas Moreau');
INSERT INTO `change_items` VALUES (10663,10577,'custom','Story Points',NULL,'3',NULL,'0.5');
INSERT INTO `change_items` VALUES (10664,10578,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10665,10579,'jira','description',NULL,'Le besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.',NULL,'h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\n*Étapes pour reproduire :*\n# Se connecter avec un compte client\n# Changer la langue en anglais\n# Ouvrir le tableau de bord\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.');
INSERT INTO `change_items` VALUES (10666,10580,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10667,10581,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10668,10581,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10669,10582,'jira','assignee',NULL,NULL,'JIRAUSER10001','Lucas Bernard');
INSERT INTO `change_items` VALUES (10670,10583,'jira','summary',NULL,'[À qualifier] Renouveler la mutuelle d\'entreprise',NULL,'Renouveler la mutuelle d\'entreprise');
INSERT INTO `change_items` VALUES (10671,10584,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10672,10585,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10673,10585,'jira','resolution',NULL,NULL,'4','Incomplete');
INSERT INTO `change_items` VALUES (10674,10586,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10675,10586,'jira','resolution',NULL,NULL,'2','Won\'t Fix');
INSERT INTO `change_items` VALUES (10676,10587,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10677,10588,'jira','Link',NULL,NULL,'OPS-45','This issue relates to OPS-45');
INSERT INTO `change_items` VALUES (10678,10589,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10679,10590,'jira','assignee','JIRAUSER10011','Julien Roux','JIRAUSER10013','Maxime Bertrand');
INSERT INTO `change_items` VALUES (10680,10591,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10681,10592,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10682,10593,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10683,10594,'jira','labels',NULL,'',NULL,'régression frontend');
INSERT INTO `change_items` VALUES (10684,10595,'jira','status','3','In Progress','10001','Done');
INSERT INTO `change_items` VALUES (10685,10595,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10686,10596,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10687,10597,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10688,10598,'jira','Fix Version',NULL,NULL,'10002','2.0');
INSERT INTO `change_items` VALUES (10689,10599,'jira','assignee',NULL,NULL,'JIRAUSER10007','Nicolas Laurent');
INSERT INTO `change_items` VALUES (10690,10600,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10691,10601,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10692,10602,'jira','Link',NULL,NULL,'DEMO-92','This issue clones DEMO-92');
INSERT INTO `change_items` VALUES (10693,10603,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10694,10603,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10695,10604,'jira','Link',NULL,NULL,'WEB-49','This issue duplicates WEB-49');
INSERT INTO `change_items` VALUES (10696,10605,'jira','labels',NULL,'',NULL,'régression mobile');
INSERT INTO `change_items` VALUES (10697,10606,'jira','summary',NULL,'[À qualifier] En tant que client, je veux suivre ma livraison',NULL,'En tant que client, je veux suivre ma livraison');
INSERT INTO `change_items` VALUES (10698,10607,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10699,10608,'jira','Link',NULL,NULL,'DEMO-55','This issue relates to DEMO-55');
INSERT INTO `change_items` VALUES (10700,10609,'jira','assignee',NULL,NULL,'JIRAUSER10011','Julien Roux');
INSERT INTO `change_items` VALUES (10701,10610,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10702,10611,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10703,10612,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10704,10613,'jira','Link',NULL,NULL,'DEMO-16','This issue relates to DEMO-16');
INSERT INTO `change_items` VALUES (10705,10614,'jira','Link',NULL,NULL,'WEB-54','This issue relates to WEB-54');
INSERT INTO `change_items` VALUES (10706,10615,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10707,10616,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10708,10617,'jira','priority','2','High','3','Medium');
INSERT INTO `change_items` VALUES (10709,10618,'jira','Attachment',NULL,NULL,'10075','export.csv');
INSERT INTO `change_items` VALUES (10710,10619,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10711,10619,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10712,10620,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10713,10621,'jira','Attachment',NULL,NULL,'10074','spec.pdf');
INSERT INTO `change_items` VALUES (10714,10622,'jira','Attachment',NULL,NULL,'10073','logs-prod.txt');
INSERT INTO `change_items` VALUES (10715,10623,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10716,10624,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10717,10625,'jira','assignee',NULL,NULL,'paul.martin','Paul Martin');
INSERT INTO `change_items` VALUES (10718,10626,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10719,10627,'jira','status','3','In Progress','10000','To Do');
INSERT INTO `change_items` VALUES (10720,10628,'jira','assignee',NULL,NULL,'jdoe','Jeanne Dupont');
INSERT INTO `change_items` VALUES (10721,10629,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10722,10630,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10723,10631,'jira','Fix Version',NULL,NULL,'10005','2025.2');
INSERT INTO `change_items` VALUES (10724,10632,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10725,10632,'jira','resolution',NULL,NULL,'2','Won\'t Fix');
INSERT INTO `change_items` VALUES (10726,10633,'jira','assignee','JIRAUSER10001','Lucas Bernard','JIRAUSER10000','Marie Durand');
INSERT INTO `change_items` VALUES (10727,10634,'jira','priority','3','Medium','2','High');
INSERT INTO `change_items` VALUES (10728,10635,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10729,10636,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10730,10636,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10731,10637,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10732,10638,'jira','Attachment',NULL,NULL,'10076','logs-prod.txt');
INSERT INTO `change_items` VALUES (10733,10639,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10734,10640,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10735,10640,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10736,10641,'jira','assignee',NULL,NULL,'JIRAUSER10004','Léa Richard');
INSERT INTO `change_items` VALUES (10737,10642,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10738,10643,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10739,10644,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10740,10644,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10741,10645,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10742,10646,'jira','status','1','Open','6','Closed');
INSERT INTO `change_items` VALUES (10743,10646,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10744,10647,'jira','Attachment',NULL,NULL,'10077','spec.pdf');
INSERT INTO `change_items` VALUES (10745,10648,'jira','Attachment',NULL,NULL,'10078','capture.png');
INSERT INTO `change_items` VALUES (10746,10649,'jira','Link',NULL,NULL,'WEB-60','This issue relates to WEB-60');
INSERT INTO `change_items` VALUES (10747,10650,'jira','priority','5','Lowest','1','Highest');
INSERT INTO `change_items` VALUES (10748,10651,'jira','Link',NULL,NULL,'DEMO-29','This issue duplicates DEMO-29');
INSERT INTO `change_items` VALUES (10749,10652,'jira','Link',NULL,NULL,'DEMO-48','This issue blocks DEMO-48');
INSERT INTO `change_items` VALUES (10750,10653,'jira','summary',NULL,'[À qualifier] Refondre la page de profil',NULL,'Refondre la page de profil');
INSERT INTO `change_items` VALUES (10751,10654,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10752,10655,'jira','summary',NULL,'[À qualifier] En tant que agent du support, je veux réinitialiser mon mot de passe',NULL,'En tant que agent du support, je veux réinitialiser mon mot de passe');
INSERT INTO `change_items` VALUES (10753,10656,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10754,10657,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10755,10658,'jira','Link',NULL,NULL,'DEMO-36','This issue blocks DEMO-36');
INSERT INTO `change_items` VALUES (10756,10659,'jira','description',NULL,'C\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.',NULL,'h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Un message clair s\'affiche en cas d\'erreur\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}\n\n[~julien.roux] peux-tu valider ?');
INSERT INTO `change_items` VALUES (10757,10660,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10758,10661,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10759,10661,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10760,10662,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10761,10663,'jira','priority','1','Highest','5','Lowest');
INSERT INTO `change_items` VALUES (10762,10664,'jira','assignee',NULL,NULL,'JIRAUSER10006','Chloé Simon');
INSERT INTO `change_items` VALUES (10763,10665,'jira','Fix Version',NULL,NULL,'10001','1.1');
INSERT INTO `change_items` VALUES (10764,10666,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10765,10667,'jira','Link',NULL,NULL,'OPS-32','This issue blocks OPS-32');
INSERT INTO `change_items` VALUES (10766,10668,'jira','assignee',NULL,NULL,'JIRAUSER10013','Maxime Bertrand');
INSERT INTO `change_items` VALUES (10767,10669,'jira','status','3','In Progress','10001','Done');
INSERT INTO `change_items` VALUES (10768,10669,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10769,10670,'jira','Link',NULL,NULL,'DEMO-22','This issue duplicates DEMO-22');
INSERT INTO `change_items` VALUES (10770,10671,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10771,10672,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10772,10673,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10773,10674,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10774,10674,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10775,10675,'jira','assignee',NULL,NULL,'JIRAUSER10012','Inès David');
INSERT INTO `change_items` VALUES (10776,10676,'jira','status','10000','To Do','10001','Done');
INSERT INTO `change_items` VALUES (10777,10676,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10778,10677,'jira','priority','2','High','3','Medium');
INSERT INTO `change_items` VALUES (10779,10678,'jira','assignee','JIRAUSER10005','Thomas Moreau','JIRAUSER10001','Lucas Bernard');
INSERT INTO `change_items` VALUES (10780,10679,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10781,10680,'jira','Attachment',NULL,NULL,'10079','logs-prod.txt');
INSERT INTO `change_items` VALUES (10782,10681,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10783,10682,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10784,10682,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10785,10683,'jira','Link',NULL,NULL,'WEB-69','This issue relates to WEB-69');
INSERT INTO `change_items` VALUES (10786,10684,'custom','Story Points',NULL,'8',NULL,'3');
INSERT INTO `change_items` VALUES (10787,10685,'jira','assignee','JIRAUSER10004','Léa Richard','JIRAUSER10006','Chloé Simon');
INSERT INTO `change_items` VALUES (10788,10686,'jira','description',NULL,'Plusieurs clients nous ont remonté le problème depuis la dernière mise en production.',NULL,'h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n* Les droits existants sont respectés\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}');
INSERT INTO `change_items` VALUES (10789,10687,'jira','Link',NULL,NULL,'DEMO-104','This issue relates to DEMO-104');
INSERT INTO `change_items` VALUES (10790,10688,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10791,10689,'jira','assignee',NULL,NULL,'JIRAUSER10000','Marie Durand');
INSERT INTO `change_items` VALUES (10792,10690,'jira','assignee',NULL,NULL,'JIRAUSER10011','Julien Roux');
INSERT INTO `change_items` VALUES (10793,10691,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10794,10691,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10795,10692,'jira','status','5','Resolved','4','Reopened');
INSERT INTO `change_items` VALUES (10796,10692,'jira','resolution','1','Fixed',NULL,NULL);
INSERT INTO `change_items` VALUES (10797,10693,'jira','priority','1','Highest','3','Medium');
INSERT INTO `change_items` VALUES (10798,10694,'jira','description',NULL,'Le besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.',NULL,'h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}\n\n!capture.png!');
INSERT INTO `change_items` VALUES (10799,10695,'jira','Attachment',NULL,NULL,'10080','capture.png');
INSERT INTO `change_items` VALUES (10800,10696,'jira','Attachment',NULL,NULL,'10081','export.csv');
INSERT INTO `change_items` VALUES (10801,10697,'jira','assignee','JIRAUSER10013','Maxime Bertrand','JIRAUSER10000','Marie Durand');
INSERT INTO `change_items` VALUES (10802,10698,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10803,10699,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10804,10700,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10805,10700,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10806,10701,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10807,10702,'jira','labels',NULL,'',NULL,'dette-technique à-valider');
INSERT INTO `change_items` VALUES (10808,10703,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10809,10704,'jira','status','3','In Progress','10000','To Do');
INSERT INTO `change_items` VALUES (10810,10705,'custom','Story Points',NULL,'1',NULL,'8');
INSERT INTO `change_items` VALUES (10811,10706,'jira','Link',NULL,NULL,'DEMO-32','This issue relates to DEMO-32');
INSERT INTO `change_items` VALUES (10812,10707,'jira','Link',NULL,NULL,'DEMO-64','This issue blocks DEMO-64');
INSERT INTO `change_items` VALUES (10813,10708,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10814,10709,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10815,10710,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10816,10710,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10817,10711,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10818,10712,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10819,10713,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10820,10713,'jira','resolution',NULL,NULL,'2','Won\'t Fix');
INSERT INTO `change_items` VALUES (10821,10714,'jira','assignee',NULL,NULL,'JIRAUSER10012','Inès David');
INSERT INTO `change_items` VALUES (10822,10715,'jira','description',NULL,'Plusieurs clients nous ont remonté le problème depuis la dernière mise en production.',NULL,'h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* La page répond en moins d\'une seconde\n\nLié à RH-21.');
INSERT INTO `change_items` VALUES (10823,10716,'jira','summary',NULL,'[À qualifier] Préparer la visite médicale de Mme Benali',NULL,'Préparer la visite médicale de Mme Benali');
INSERT INTO `change_items` VALUES (10824,10717,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10825,10718,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10826,10718,'jira','resolution',NULL,NULL,'2','Won\'t Fix');
INSERT INTO `change_items` VALUES (10827,10719,'jira','Link',NULL,NULL,'RH-15','This issue blocks RH-15');
INSERT INTO `change_items` VALUES (10828,10720,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10829,10721,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10830,10722,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10831,10722,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10832,10723,'jira','priority','4','Low','2','High');
INSERT INTO `change_items` VALUES (10833,10724,'jira','Attachment',NULL,NULL,'10082','export.csv');
INSERT INTO `change_items` VALUES (10834,10725,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10835,10726,'jira','Attachment',NULL,NULL,'10083','logs-prod.txt');
INSERT INTO `change_items` VALUES (10836,10727,'jira','Attachment',NULL,NULL,'10084','logs-prod.txt');
INSERT INTO `change_items` VALUES (10837,10728,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10838,10729,'jira','Attachment',NULL,NULL,'10085','spec.pdf');
INSERT INTO `change_items` VALUES (10839,10730,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10840,10730,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10841,10731,'jira','priority','1','Highest','2','High');
INSERT INTO `change_items` VALUES (10842,10732,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10843,10733,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10844,10734,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10845,10734,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10846,10735,'jira','Attachment',NULL,NULL,'10086','export.csv');
INSERT INTO `change_items` VALUES (10847,10736,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10848,10737,'jira','Attachment',NULL,NULL,'10087','logs-prod.txt');
INSERT INTO `change_items` VALUES (10849,10738,'jira','Attachment',NULL,NULL,'10088','maquette.jpg');
INSERT INTO `change_items` VALUES (10850,10739,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10851,10739,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10852,10740,'jira','priority','4','Low','2','High');
INSERT INTO `change_items` VALUES (10853,10741,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10854,10742,'jira','Link',NULL,NULL,'WEB-73','This issue relates to WEB-73');
INSERT INTO `change_items` VALUES (10855,10743,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10856,10743,'jira','resolution',NULL,NULL,'3','Duplicate');
INSERT INTO `change_items` VALUES (10857,10744,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10858,10745,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10859,10746,'jira','Link',NULL,NULL,'WEB-25','This issue relates to WEB-25');
INSERT INTO `change_items` VALUES (10860,10747,'jira','summary',NULL,'[À qualifier] Documenter le système de notifications',NULL,'Documenter le système de notifications');
INSERT INTO `change_items` VALUES (10861,10748,'jira','summary',NULL,'[À qualifier] Déployer en préproduction',NULL,'Déployer en préproduction');
INSERT INTO `change_items` VALUES (10862,10749,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10863,10750,'jira','status','3','In Progress','10001','Done');
INSERT INTO `change_items` VALUES (10864,10750,'jira','resolution',NULL,NULL,'2','Won\'t Fix');
INSERT INTO `change_items` VALUES (10865,10751,'jira','summary',NULL,'[À qualifier] Augmenter l\'espace disque sur srv-web-01',NULL,'Augmenter l\'espace disque sur srv-web-01');
INSERT INTO `change_items` VALUES (10866,10752,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10867,10753,'jira','description',NULL,'Le besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.',NULL,'h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n* Les droits existants sont respectés\n\nLié à RH-10.');
INSERT INTO `change_items` VALUES (10868,10754,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10869,10754,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10870,10755,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10871,10756,'custom','Story Points',NULL,'13',NULL,'8');
INSERT INTO `change_items` VALUES (10872,10757,'jira','status','10000','To Do','10001','Done');
INSERT INTO `change_items` VALUES (10873,10757,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10874,10758,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10875,10759,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10876,10759,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10877,10760,'jira','priority','1','Highest','5','Lowest');
INSERT INTO `change_items` VALUES (10878,10761,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10879,10762,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10880,10763,'jira','Fix Version',NULL,NULL,'10001','1.1');
INSERT INTO `change_items` VALUES (10881,10764,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10882,10764,'jira','resolution',NULL,NULL,'3','Duplicate');
INSERT INTO `change_items` VALUES (10883,10765,'jira','description',NULL,'C\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.',NULL,'h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Le résultat est identique sur mobile et sur ordinateur\n* Les droits existants sont respectés\n\n[~marie.durand] peux-tu valider ?');
INSERT INTO `change_items` VALUES (10884,10766,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10885,10767,'jira','Fix Version',NULL,NULL,'10004','2025.1');
INSERT INTO `change_items` VALUES (10886,10768,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10887,10769,'jira','assignee',NULL,NULL,'JIRAUSER10019','Yann Lecomte');
INSERT INTO `change_items` VALUES (10888,10770,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10889,10771,'jira','labels',NULL,'',NULL,'à-valider régression perf');
INSERT INTO `change_items` VALUES (10890,10772,'jira','status','3','In Progress','10001','Done');
INSERT INTO `change_items` VALUES (10891,10772,'jira','resolution',NULL,NULL,'3','Duplicate');
INSERT INTO `change_items` VALUES (10892,10773,'jira','assignee',NULL,NULL,'JIRAUSER10002','Camille Petit');
INSERT INTO `change_items` VALUES (10893,10774,'jira','priority','5','Lowest','3','Medium');
INSERT INTO `change_items` VALUES (10894,10775,'jira','status','1','Open','5','Resolved');
INSERT INTO `change_items` VALUES (10895,10775,'jira','resolution',NULL,NULL,'4','Incomplete');
INSERT INTO `change_items` VALUES (10896,10776,'jira','Attachment',NULL,NULL,'10089','export.csv');
INSERT INTO `change_items` VALUES (10897,10777,'jira','assignee',NULL,NULL,'JIRAUSER10017','Karim Benali');
INSERT INTO `change_items` VALUES (10898,10778,'jira','Link',NULL,NULL,'DEMO-52','This issue relates to DEMO-52');
INSERT INTO `change_items` VALUES (10899,10779,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10900,10780,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10901,10780,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10902,10781,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10903,10782,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10904,10783,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10905,10783,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10906,10784,'jira','Link',NULL,NULL,'WEB-48','This issue duplicates WEB-48');
INSERT INTO `change_items` VALUES (10907,10785,'custom','Story Points',NULL,'2',NULL,'1');
INSERT INTO `change_items` VALUES (10908,10786,'jira','description',NULL,'Le besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.',NULL,'h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Les droits existants sont respectés\n* Le résultat est identique sur mobile et sur ordinateur\n\nVoir [la maquette|https://maquettes.exemple.fr/panier].');
INSERT INTO `change_items` VALUES (10909,10787,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10910,10788,'jira','Attachment',NULL,NULL,'10091','export.csv');
INSERT INTO `change_items` VALUES (10911,10789,'jira','Attachment',NULL,NULL,'10090','capture.png');
INSERT INTO `change_items` VALUES (10912,10790,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10913,10791,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10914,10791,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10915,10792,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10916,10793,'jira','status','3','In Progress','10001','Done');
INSERT INTO `change_items` VALUES (10917,10793,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10918,10794,'jira','priority','2','High','4','Low');
INSERT INTO `change_items` VALUES (10919,10795,'jira','assignee','JIRAUSER10009','Antoine Lefebvre','JIRAUSER10012','Inès David');
INSERT INTO `change_items` VALUES (10920,10796,'jira','labels',NULL,'',NULL,'backend');
INSERT INTO `change_items` VALUES (10921,10797,'jira','Attachment',NULL,NULL,'10092','logs-prod.txt');
INSERT INTO `change_items` VALUES (10922,10798,'jira','Attachment',NULL,NULL,'10094','maquette.jpg');
INSERT INTO `change_items` VALUES (10923,10799,'jira','Attachment',NULL,NULL,'10093','spec.pdf');
INSERT INTO `change_items` VALUES (10924,10800,'jira','assignee','paul.martin','Paul Martin','JIRAUSER10000','Marie Durand');
INSERT INTO `change_items` VALUES (10925,10801,'jira','priority','4','Low','3','Medium');
INSERT INTO `change_items` VALUES (10926,10802,'jira','Attachment',NULL,NULL,'10095','capture.png');
INSERT INTO `change_items` VALUES (10927,10803,'jira','Attachment',NULL,NULL,'10097','maquette.jpg');
INSERT INTO `change_items` VALUES (10928,10804,'jira','Attachment',NULL,NULL,'10096','export.csv');
INSERT INTO `change_items` VALUES (10929,10805,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10930,10806,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10931,10806,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10932,10807,'jira','assignee','JIRAUSER10002','Camille Petit','JIRAUSER10012','Inès David');
INSERT INTO `change_items` VALUES (10933,10808,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10934,10809,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10935,10810,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10936,10810,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10937,10811,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10938,10812,'jira','status','3','In Progress','10000','To Do');
INSERT INTO `change_items` VALUES (10939,10813,'jira','assignee','JIRAUSER10011','Julien Roux','paul.martin','Paul Martin');
INSERT INTO `change_items` VALUES (10940,10814,'jira','Link',NULL,NULL,'WEB-3','This issue blocks WEB-3');
INSERT INTO `change_items` VALUES (10941,10815,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10942,10816,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10943,10817,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10944,10817,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10945,10818,'jira','assignee','JIRAUSER10012','Inès David','JIRAUSER10000','Marie Durand');
INSERT INTO `change_items` VALUES (10946,10819,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10947,10820,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10948,10821,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10949,10821,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10950,10822,'custom','Story Points',NULL,'1',NULL,'3');
INSERT INTO `change_items` VALUES (10951,10823,'jira','priority','5','Lowest','3','Medium');
INSERT INTO `change_items` VALUES (10952,10824,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10953,10825,'jira','description',NULL,'La supervision a levé l\'alerte trois nuits de suite.',NULL,'h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}\n\n[~ines.david] peux-tu valider ?');
INSERT INTO `change_items` VALUES (10954,10826,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10955,10827,'jira','Link',NULL,NULL,'OPS-45','This issue blocks OPS-45');
INSERT INTO `change_items` VALUES (10956,10828,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10957,10828,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10958,10829,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10959,10830,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10960,10831,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10961,10831,'jira','resolution',NULL,NULL,'1','Fixed');
INSERT INTO `change_items` VALUES (10962,10832,'jira','status','5','Resolved','6','Closed');
INSERT INTO `change_items` VALUES (10963,10833,'jira','labels',NULL,'',NULL,'ux');
INSERT INTO `change_items` VALUES (10964,10834,'jira','summary',NULL,'[À qualifier] Mettre à jour la documentation',NULL,'Mettre à jour la documentation');
INSERT INTO `change_items` VALUES (10965,10835,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10966,10836,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10967,10837,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10968,10837,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10969,10838,'jira','Attachment',NULL,NULL,'10099','spec.pdf');
INSERT INTO `change_items` VALUES (10970,10839,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10971,10840,'custom','Story Points',NULL,'8',NULL,'2');
INSERT INTO `change_items` VALUES (10972,10841,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10973,10842,'jira','assignee',NULL,NULL,'JIRAUSER10000','Marie Durand');
INSERT INTO `change_items` VALUES (10974,10843,'jira','Attachment',NULL,NULL,'10098','capture.png');
INSERT INTO `change_items` VALUES (10975,10844,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10976,10844,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10977,10845,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10978,10846,'jira','priority','3','Medium','5','Lowest');
INSERT INTO `change_items` VALUES (10979,10847,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10980,10848,'jira','description',NULL,'C\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.',NULL,'h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\n*Étapes pour reproduire :*\n# Rafraîchir la page\n# Ajouter deux articles au panier\n# Se connecter avec un compte client\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\nLié à DEMO-35.');
INSERT INTO `change_items` VALUES (10981,10849,'jira','status','3','In Progress','10001','Done');
INSERT INTO `change_items` VALUES (10982,10849,'jira','resolution',NULL,NULL,'2','Won\'t Fix');
INSERT INTO `change_items` VALUES (10983,10850,'jira','Link',NULL,NULL,'DEMO-80','This issue relates to DEMO-80');
INSERT INTO `change_items` VALUES (10984,10851,'jira','status','10000','To Do','3','In Progress');
INSERT INTO `change_items` VALUES (10985,10852,'jira','assignee','JIRAUSER10005','Thomas Moreau','JIRAUSER10013','Maxime Bertrand');
INSERT INTO `change_items` VALUES (10986,10853,'jira','status','3','In Progress','10002','In Review');
INSERT INTO `change_items` VALUES (10987,10854,'jira','status','10002','In Review','10001','Done');
INSERT INTO `change_items` VALUES (10988,10854,'jira','resolution',NULL,NULL,'10000','Done');
INSERT INTO `change_items` VALUES (10989,10855,'jira','status','1','Open','3','In Progress');
INSERT INTO `change_items` VALUES (10990,10856,'jira','summary',NULL,'[À qualifier] Planifier la formation sécurité',NULL,'Planifier la formation sécurité');
INSERT INTO `change_items` VALUES (10991,10857,'jira','status','3','In Progress','5','Resolved');
INSERT INTO `change_items` VALUES (10992,10857,'jira','resolution',NULL,NULL,'1','Fixed');
/*!40000 ALTER TABLE `change_items` ENABLE KEYS */;
DROP TABLE IF EXISTS `comments`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `comments` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `issue_id` bigint(20) NOT NULL,
  `author_id` bigint(20) DEFAULT NULL,
  `body` longtext NOT NULL,
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  `updated_by_id` bigint(20) DEFAULT NULL,
  `visibility_group` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin DEFAULT NULL,
  `visibility_role_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_comments_issue` (`issue_id`,`created_at`),
  KEY `fk_comments_author` (`author_id`),
  KEY `fk_comments_updated_by` (`updated_by_id`),
  KEY `fk_comments_role` (`visibility_role_id`),
  CONSTRAINT `fk_comments_author` FOREIGN KEY (`author_id`) REFERENCES `users` (`id`) ON DELETE SET NULL,
  CONSTRAINT `fk_comments_issue` FOREIGN KEY (`issue_id`) REFERENCES `issues` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_comments_role` FOREIGN KEY (`visibility_role_id`) REFERENCES `project_roles` (`id`),
  CONSTRAINT `fk_comments_updated_by` FOREIGN KEY (`updated_by_id`) REFERENCES `users` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB AUTO_INCREMENT=10607 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `comments` DISABLE KEYS */;
INSERT INTO `comments` VALUES (10000,10000,2,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2025-09-11 11:25:37.000','2025-09-11 11:25:37.000',2,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10001,10001,7,'Même cause que DEMO-1, je fais le lien.','2025-09-05 08:00:15.000','2025-09-05 08:00:15.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10002,10001,3,'C\'est bon pour moi, on peut fermer.','2025-09-11 13:50:13.000','2025-09-11 13:50:13.000',3,NULL,NULL);
INSERT INTO `comments` VALUES (10003,10001,12,'[~lucas.bernard] tu peux regarder quand tu as un moment ?','2025-09-15 13:28:19.000','2025-09-15 13:28:19.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10004,10001,2,'[~marie.durand] tu peux regarder quand tu as un moment ?','2025-09-21 12:20:57.000','2025-09-21 12:20:57.000',2,NULL,10002);
INSERT INTO `comments` VALUES (10005,10001,24,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-09-24 15:27:20.000','2025-09-24 15:27:20.000',24,NULL,NULL);
INSERT INTO `comments` VALUES (10006,10001,15,'Point fait avec le client ce matin, il valide la solution proposée.','2025-09-30 14:06:58.000','2025-09-30 14:06:58.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10007,10002,2,'C\'est bon pour moi, on peut fermer.','2025-09-13 11:23:43.000','2025-09-13 11:23:43.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10008,10002,6,'Je confirme, reproduit sur l\'environnement de recette.','2025-09-16 17:11:19.000','2025-09-16 17:11:19.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10009,10002,16,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2025-09-20 14:23:40.000','2025-09-20 14:23:40.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10010,10003,5,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2025-09-11 17:59:18.000','2025-09-11 17:59:18.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10011,10004,11,'Je confirme, reproduit sur l\'environnement de recette.','2025-09-19 07:34:53.000','2025-09-19 07:34:53.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10012,10004,10,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2025-09-28 06:26:36.000','2025-09-28 06:26:36.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10013,10005,2,'Je confirme, reproduit sur l\'environnement de recette.','2025-09-19 15:57:04.000','2025-09-19 15:57:04.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10014,10007,17,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2025-09-19 17:45:37.000','2025-09-19 17:45:37.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10015,10007,9,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-09-26 10:05:26.000','2025-09-26 10:05:26.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10016,10008,17,'Je confirme, reproduit sur l\'environnement de recette.','2025-10-01 07:01:45.000','2025-10-01 07:01:45.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10017,10008,16,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-10-12 17:29:10.000','2025-10-12 17:29:10.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10018,10010,6,'[~maxime.bertrand] tu peux regarder quand tu as un moment ?','2025-09-24 14:01:29.000','2025-09-24 14:01:29.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10019,10010,24,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2025-09-24 16:59:42.000','2025-09-24 16:59:42.000',24,NULL,NULL);
INSERT INTO `comments` VALUES (10020,10010,15,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-10-08 08:29:07.000','2025-10-08 08:29:07.000',15,NULL,10002);
INSERT INTO `comments` VALUES (10021,10011,17,'[~julien.roux] tu peux regarder quand tu as un moment ?','2025-10-04 11:57:51.000','2025-10-04 11:57:51.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10022,10011,11,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2025-10-06 08:08:02.000','2025-10-06 08:08:02.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10023,10011,19,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2025-10-19 08:34:01.000','2025-10-19 08:34:01.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10024,10011,3,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-11-01 13:03:54.000','2025-11-01 13:03:54.000',3,NULL,NULL);
INSERT INTO `comments` VALUES (10025,10012,12,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2025-10-03 12:19:05.000','2025-10-03 12:19:05.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10026,10012,12,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2025-10-11 16:54:26.000','2025-10-11 16:54:26.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10027,10012,20,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2025-10-14 09:34:48.000','2025-10-14 09:34:48.000',20,NULL,NULL);
INSERT INTO `comments` VALUES (10028,10014,8,'C\'est bon pour moi, on peut fermer.','2025-10-01 16:38:46.000','2025-10-01 16:38:46.000',8,NULL,NULL);
INSERT INTO `comments` VALUES (10029,10014,14,'Point fait avec le client ce matin, il valide la solution proposée.','2025-10-02 17:33:16.000','2025-10-02 17:33:16.000',14,NULL,NULL);
INSERT INTO `comments` VALUES (10030,10014,14,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2025-10-17 14:47:06.000','2025-10-17 14:47:06.000',14,NULL,NULL);
INSERT INTO `comments` VALUES (10031,10014,8,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2025-10-19 14:46:12.000','2025-10-19 14:46:12.000',8,NULL,NULL);
INSERT INTO `comments` VALUES (10032,10014,10,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2025-11-02 16:20:06.000','2025-11-02 16:20:06.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10033,10014,8,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2025-11-15 16:05:35.000','2025-11-15 17:02:56.000',8,NULL,NULL);
INSERT INTO `comments` VALUES (10034,10015,9,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2025-10-02 07:09:29.000','2025-10-02 07:09:29.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10035,10015,11,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2025-10-16 15:44:28.000','2025-10-16 15:44:28.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10036,10015,11,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2025-10-17 10:37:02.000','2025-10-17 10:37:02.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10037,10015,2,'[~camille.petit] tu peux regarder quand tu as un moment ?','2025-10-18 06:57:57.000','2025-10-18 06:57:57.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10038,10017,16,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2025-10-06 17:30:21.000','2025-10-06 17:30:21.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10039,10017,16,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2025-10-19 16:01:57.000','2025-10-19 16:01:57.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10040,10017,11,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2025-11-03 18:31:04.000','2025-11-03 18:31:04.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10041,10020,11,'Je confirme, reproduit sur l\'environnement de recette.','2025-10-27 07:02:05.000','2025-10-27 07:02:05.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10042,10020,7,'Je confirme, reproduit sur l\'environnement de recette.','2025-11-06 07:48:52.000','2025-11-06 07:48:52.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10043,10020,2,'Je confirme, reproduit sur l\'environnement de recette.','2025-11-10 08:49:02.000','2025-11-10 08:49:02.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10044,10020,7,'Je confirme, reproduit sur l\'environnement de recette.','2025-11-17 17:04:22.000','2025-11-17 17:04:22.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10045,10023,9,'C\'est bon pour moi, on peut fermer.','2025-10-28 08:04:21.000','2025-10-28 08:04:21.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10046,10023,20,'Point fait avec le client ce matin, il valide la solution proposée.','2025-11-12 14:46:12.000','2025-11-12 14:46:12.000',20,NULL,NULL);
INSERT INTO `comments` VALUES (10047,10023,7,'Même cause que WEB-1, je fais le lien.','2025-11-12 18:49:42.000','2025-11-13 16:05:33.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10048,10025,13,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2025-10-31 13:56:50.000','2025-10-31 13:56:50.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10049,10025,24,'Point fait avec le client ce matin, il valide la solution proposée.','2025-11-06 10:18:46.000','2025-11-06 10:18:46.000',24,NULL,NULL);
INSERT INTO `comments` VALUES (10050,10025,10,'[~paul.martin] tu peux regarder quand tu as un moment ?','2025-11-10 10:44:23.000','2025-11-10 10:44:23.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10051,10025,13,'Je confirme, reproduit sur l\'environnement de recette.','2025-11-15 08:20:45.000','2025-11-15 08:20:45.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10052,10025,24,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-11-16 16:32:38.000','2025-11-16 16:32:38.000',24,NULL,NULL);
INSERT INTO `comments` VALUES (10053,10025,17,'C\'est bon pour moi, on peut fermer.','2025-11-24 10:55:43.000','2025-11-24 10:55:43.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10054,10029,11,'Je confirme, reproduit sur l\'environnement de recette.','2025-11-03 14:27:32.000','2025-11-03 14:27:32.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10055,10029,19,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2025-11-07 18:38:15.000','2025-11-07 18:38:15.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10056,10030,24,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-10-30 15:58:09.000','2025-10-30 15:58:09.000',24,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10057,10032,2,'C\'est bon pour moi, on peut fermer.','2025-10-31 16:08:24.000','2025-10-31 16:08:24.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10058,10032,11,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2025-11-13 17:19:20.000','2025-11-13 17:19:20.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10059,10033,11,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2025-11-03 12:47:28.000','2025-11-03 12:47:28.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10060,10034,16,'Je confirme, reproduit sur l\'environnement de recette.','2025-11-08 13:07:37.000','2025-11-08 13:07:37.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10061,10034,2,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2025-11-17 09:59:13.000','2025-11-17 09:59:13.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10062,10035,10,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-11-07 13:13:03.000','2025-11-07 14:37:22.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10063,10036,11,'C\'est bon pour moi, on peut fermer.','2025-11-06 11:33:59.000','2025-11-06 11:33:59.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10064,10037,7,'[~thomas.moreau] tu peux regarder quand tu as un moment ?','2025-11-06 16:54:33.000','2025-11-06 16:54:33.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10065,10038,24,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2025-11-06 14:45:39.000','2025-11-06 14:45:39.000',24,NULL,NULL);
INSERT INTO `comments` VALUES (10066,10038,11,'C\'est bon pour moi, on peut fermer.','2025-11-21 14:39:39.000','2025-11-21 14:39:39.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10067,10039,4,'Point fait avec le client ce matin, il valide la solution proposée.','2025-11-06 18:59:11.000','2025-11-06 18:59:11.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10068,10039,15,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2025-11-20 12:31:31.000','2025-11-20 12:31:31.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10069,10039,17,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-11-23 10:07:26.000','2025-11-23 10:07:26.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10070,10040,16,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2025-11-22 15:14:16.000','2025-11-22 15:14:16.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10071,10040,15,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2025-11-26 09:09:31.000','2025-11-26 09:09:31.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10072,10040,12,'Je confirme, reproduit sur l\'environnement de recette.','2025-11-28 07:04:16.000','2025-11-28 07:04:16.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10073,10040,19,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2025-11-28 14:09:16.000','2025-11-28 14:44:34.000',19,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10074,10040,13,'[~ines.david] tu peux regarder quand tu as un moment ?','2025-12-11 18:03:59.000','2025-12-11 18:03:59.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10075,10040,18,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-12-18 07:42:28.000','2025-12-18 07:42:28.000',18,NULL,NULL);
INSERT INTO `comments` VALUES (10076,10041,9,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2025-11-15 12:18:02.000','2025-11-17 07:30:31.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10077,10041,13,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-11-30 11:07:32.000','2025-11-30 11:07:32.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10078,10041,17,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2025-12-02 11:16:30.000','2025-12-02 11:16:30.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10079,10042,5,'Point fait avec le client ce matin, il valide la solution proposée.','2025-11-12 10:12:28.000','2025-11-12 10:12:28.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10080,10042,20,'Point fait avec le client ce matin, il valide la solution proposée.','2025-11-22 10:00:22.000','2025-11-22 10:00:22.000',20,NULL,NULL);
INSERT INTO `comments` VALUES (10081,10042,5,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2025-11-30 10:11:51.000','2025-11-30 10:11:51.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10082,10042,4,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2025-12-03 09:18:55.000','2025-12-03 09:18:55.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10083,10042,4,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2025-12-06 07:10:49.000','2025-12-06 07:10:49.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10084,10042,2,'Même cause que WEB-7, je fais le lien.','2025-12-21 15:22:48.000','2025-12-21 15:22:48.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10085,10043,24,'Point fait avec le client ce matin, il valide la solution proposée.','2025-11-17 14:32:32.000','2025-11-17 14:32:32.000',24,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10086,10043,23,'Même cause que OPS-2, je fais le lien.','2025-11-21 08:51:44.000','2025-11-24 17:56:46.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10087,10044,16,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2025-11-14 16:01:09.000','2025-11-15 12:25:25.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10088,10045,16,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-11-16 16:35:13.000','2025-11-16 16:35:13.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10089,10045,2,'[~hugo.robert] tu peux regarder quand tu as un moment ?','2025-11-23 13:36:28.000','2025-11-23 13:36:28.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10090,10046,10,'Même cause que DEMO-21, je fais le lien.','2025-11-18 15:12:31.000','2025-11-18 15:12:31.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10091,10046,3,'Je confirme, reproduit sur l\'environnement de recette.','2025-11-29 07:51:26.000','2025-11-29 07:51:26.000',3,NULL,NULL);
INSERT INTO `comments` VALUES (10092,10046,17,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2025-12-09 16:08:16.000','2025-12-09 16:08:16.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10093,10047,24,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2025-11-29 18:21:56.000','2025-11-29 18:21:56.000',24,NULL,NULL);
INSERT INTO `comments` VALUES (10094,10047,19,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2025-12-05 11:00:46.000','2025-12-05 11:00:46.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10095,10047,19,'Même cause que OPS-6, je fais le lien.','2025-12-14 07:07:17.000','2025-12-14 07:07:17.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10096,10047,24,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2025-12-23 15:34:27.000','2025-12-23 15:34:27.000',24,NULL,NULL);
INSERT INTO `comments` VALUES (10097,10047,9,'C\'est bon pour moi, on peut fermer.','2025-12-27 16:59:34.000','2025-12-27 16:59:34.000',9,NULL,10002);
INSERT INTO `comments` VALUES (10098,10048,22,'C\'est bon pour moi, on peut fermer.','2025-12-02 13:12:12.000','2025-12-02 13:12:12.000',22,NULL,NULL);
INSERT INTO `comments` VALUES (10099,10048,24,'Point fait avec le client ce matin, il valide la solution proposée.','2025-12-13 09:51:23.000','2025-12-13 09:51:23.000',24,NULL,NULL);
INSERT INTO `comments` VALUES (10100,10048,23,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2025-12-15 07:14:11.000','2025-12-15 07:14:11.000',23,NULL,10002);
INSERT INTO `comments` VALUES (10101,10048,15,'[~camille.petit] tu peux regarder quand tu as un moment ?','2025-12-25 17:19:13.000','2025-12-25 17:19:13.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10102,10049,10,'C\'est bon pour moi, on peut fermer.','2025-11-22 13:54:12.000','2025-11-22 13:54:12.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10103,10049,9,'C\'est bon pour moi, on peut fermer.','2025-12-04 13:32:24.000','2025-12-04 13:32:24.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10104,10049,10,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2025-12-14 18:47:00.000','2025-12-14 18:47:00.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10105,10051,19,'Je confirme, reproduit sur l\'environnement de recette.','2025-11-20 10:56:20.000','2025-11-20 10:56:20.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10106,10051,24,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-11-22 07:49:27.000','2025-11-22 07:49:27.000',24,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10107,10052,7,'Je confirme, reproduit sur l\'environnement de recette.','2025-11-23 15:11:29.000','2025-11-23 15:11:29.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10108,10053,21,'[~antoine.lefebvre] tu peux regarder quand tu as un moment ?','2025-11-21 15:53:58.000','2025-11-21 15:53:58.000',21,NULL,NULL);
INSERT INTO `comments` VALUES (10109,10054,16,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-12-04 08:35:33.000','2025-12-04 08:35:33.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10110,10054,5,'Je confirme, reproduit sur l\'environnement de recette.','2025-12-12 16:44:14.000','2025-12-12 16:44:14.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10111,10054,16,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2025-12-24 15:06:28.000','2025-12-24 15:06:28.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10112,10054,7,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-01-06 18:52:58.000','2026-01-06 18:52:58.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10113,10055,17,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2025-12-07 16:29:22.000','2025-12-08 07:28:28.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10114,10055,17,'Même cause que WEB-7, je fais le lien.','2025-12-21 08:01:24.000','2025-12-21 08:01:24.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10115,10055,17,'Point fait avec le client ce matin, il valide la solution proposée.','2025-12-29 12:18:02.000','2025-12-29 12:18:02.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10116,10055,13,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-12-31 08:59:30.000','2025-12-31 08:59:30.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10117,10055,10,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-01-01 07:26:23.000','2026-01-01 07:26:23.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10118,10056,11,'C\'est bon pour moi, on peut fermer.','2025-12-08 13:53:24.000','2025-12-08 13:53:24.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10119,10057,11,'[~marie.durand] tu peux regarder quand tu as un moment ?','2025-12-10 10:09:25.000','2025-12-10 10:09:25.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10120,10057,15,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-12-24 12:37:16.000','2025-12-24 12:37:16.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10121,10057,22,'C\'est bon pour moi, on peut fermer.','2026-01-07 14:23:14.000','2026-01-07 14:23:14.000',22,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10122,10057,9,'Je confirme, reproduit sur l\'environnement de recette.','2026-01-12 09:25:46.000','2026-01-13 11:28:57.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10123,10058,7,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-12-12 15:29:09.000','2025-12-12 15:29:09.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10124,10058,19,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2025-12-16 15:03:52.000','2025-12-16 15:03:52.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10125,10058,10,'Point fait avec le client ce matin, il valide la solution proposée.','2025-12-26 13:17:53.000','2025-12-26 13:17:53.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10126,10059,9,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2025-11-29 16:00:36.000','2025-11-29 16:00:36.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10127,10059,5,'[~maxime.bertrand] tu peux regarder quand tu as un moment ?','2025-12-11 18:47:11.000','2025-12-11 18:47:11.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10128,10060,18,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2025-12-01 18:22:28.000','2025-12-01 18:22:28.000',18,NULL,NULL);
INSERT INTO `comments` VALUES (10129,10064,16,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-12-17 09:52:53.000','2025-12-17 09:52:53.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10130,10065,2,'C\'est bon pour moi, on peut fermer.','2025-12-17 11:07:25.000','2025-12-17 11:07:25.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10131,10066,7,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-12-10 14:01:57.000','2025-12-10 14:01:57.000',7,NULL,10002);
INSERT INTO `comments` VALUES (10132,10067,23,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-12-15 12:16:52.000','2025-12-15 12:16:52.000',23,NULL,NULL);
INSERT INTO `comments` VALUES (10133,10067,7,'Point fait avec le client ce matin, il valide la solution proposée.','2025-12-23 14:17:29.000','2025-12-23 14:17:29.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10134,10067,7,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-01-01 11:52:16.000','2026-01-01 11:52:16.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10135,10068,7,'[~maxime.bertrand] tu peux regarder quand tu as un moment ?','2025-12-17 15:21:09.000','2025-12-17 15:21:09.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10136,10068,7,'Je confirme, reproduit sur l\'environnement de recette.','2025-12-20 11:27:50.000','2025-12-20 11:27:50.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10137,10069,4,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2025-12-23 15:16:48.000','2025-12-23 15:16:48.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10138,10069,17,'[~thomas.moreau] tu peux regarder quand tu as un moment ?','2025-12-29 14:24:14.000','2025-12-29 14:24:14.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10139,10069,4,'Je confirme, reproduit sur l\'environnement de recette.','2026-01-02 16:33:15.000','2026-01-02 16:33:15.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10140,10069,3,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-01-15 08:08:40.000','2026-01-15 08:08:40.000',3,NULL,NULL);
INSERT INTO `comments` VALUES (10141,10069,4,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-01-30 16:37:15.000','2026-01-30 16:37:15.000',4,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10142,10069,4,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-01-31 09:22:21.000','2026-01-31 09:22:21.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10143,10073,19,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-12-13 09:41:45.000','2025-12-13 09:41:45.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10144,10073,16,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2025-12-21 17:41:59.000','2025-12-21 17:41:59.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10145,10073,19,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-01-04 10:19:31.000','2026-01-04 10:19:31.000',19,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10146,10073,13,'Je confirme, reproduit sur l\'environnement de recette.','2026-01-17 13:56:34.000','2026-01-17 13:56:34.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10147,10073,3,'Je confirme, reproduit sur l\'environnement de recette.','2026-01-21 14:35:57.000','2026-01-21 14:35:57.000',3,NULL,NULL);
INSERT INTO `comments` VALUES (10148,10074,13,'Même cause que DEMO-12, je fais le lien.','2025-12-21 07:38:47.000','2025-12-21 07:38:47.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10149,10074,13,'Point fait avec le client ce matin, il valide la solution proposée.','2025-12-26 17:14:18.000','2025-12-26 17:14:18.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10150,10074,4,'Je confirme, reproduit sur l\'environnement de recette.','2026-01-08 16:04:23.000','2026-01-08 16:04:23.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10151,10074,13,'Je confirme, reproduit sur l\'environnement de recette.','2026-01-17 07:56:40.000','2026-01-17 07:56:40.000',13,NULL,10002);
INSERT INTO `comments` VALUES (10152,10075,3,'Même cause que DEMO-1, je fais le lien.','2025-12-29 13:32:46.000','2025-12-29 13:32:46.000',3,NULL,NULL);
INSERT INTO `comments` VALUES (10153,10076,7,'Même cause que WEB-15, je fais le lien.','2025-12-21 08:01:03.000','2025-12-21 08:01:03.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10154,10076,19,'[~julien.roux] tu peux regarder quand tu as un moment ?','2025-12-21 17:48:31.000','2025-12-21 17:48:31.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10155,10076,2,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2025-12-21 18:30:14.000','2025-12-21 18:30:14.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10156,10077,3,'Point fait avec le client ce matin, il valide la solution proposée.','2025-12-16 15:36:26.000','2025-12-16 15:36:26.000',3,NULL,NULL);
INSERT INTO `comments` VALUES (10157,10077,17,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2025-12-27 10:11:33.000','2025-12-27 10:11:33.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10158,10077,2,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-12-27 12:05:01.000','2025-12-27 12:05:01.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10159,10079,10,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2025-12-24 18:12:23.000','2025-12-24 18:12:23.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10160,10079,8,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2025-12-30 16:45:44.000','2025-12-30 16:45:44.000',8,NULL,NULL);
INSERT INTO `comments` VALUES (10161,10079,14,'Même cause que RH-2, je fais le lien.','2025-12-31 11:57:09.000','2025-12-31 18:10:38.000',14,NULL,NULL);
INSERT INTO `comments` VALUES (10162,10081,17,'Même cause que WEB-12, je fais le lien.','2026-01-05 18:18:05.000','2026-01-05 18:18:05.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10163,10081,19,'[~paul.martin] tu peux regarder quand tu as un moment ?','2026-01-11 12:10:18.000','2026-01-11 12:10:18.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10164,10081,9,'[~thomas.moreau] tu peux regarder quand tu as un moment ?','2026-01-24 17:26:32.000','2026-01-24 17:26:32.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10165,10081,4,'[~ines.david] tu peux regarder quand tu as un moment ?','2026-01-28 14:17:49.000','2026-01-30 07:15:24.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10166,10081,12,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-01-28 15:10:45.000','2026-01-28 15:10:45.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10167,10082,10,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2025-12-28 08:38:55.000','2025-12-28 08:38:55.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10168,10082,10,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2025-12-29 17:06:33.000','2025-12-29 17:43:57.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10169,10083,7,'Même cause que WEB-5, je fais le lien.','2025-12-29 18:21:09.000','2025-12-29 18:21:09.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10170,10083,4,'Point fait avec le client ce matin, il valide la solution proposée.','2025-12-29 18:31:17.000','2025-12-29 18:31:17.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10171,10083,6,'Je confirme, reproduit sur l\'environnement de recette.','2026-01-05 16:21:24.000','2026-01-05 16:21:24.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10172,10083,2,'[~nicolas.laurent] tu peux regarder quand tu as un moment ?','2026-01-05 16:59:00.000','2026-01-05 16:59:00.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10173,10084,2,'Point fait avec le client ce matin, il valide la solution proposée.','2026-01-12 17:05:34.000','2026-01-12 17:05:34.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10174,10084,18,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-01-16 10:32:18.000','2026-01-16 10:32:18.000',18,NULL,NULL);
INSERT INTO `comments` VALUES (10175,10084,9,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-01-24 08:39:30.000','2026-01-24 08:39:30.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10176,10086,2,'C\'est bon pour moi, on peut fermer.','2026-01-10 10:52:03.000','2026-01-10 16:06:56.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10177,10086,12,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-01-18 14:42:40.000','2026-01-18 14:42:40.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10178,10087,6,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-01-14 11:48:06.000','2026-01-14 11:48:06.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10179,10087,4,'Même cause que DEMO-11, je fais le lien.','2026-01-29 10:56:28.000','2026-01-29 10:56:28.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10180,10087,17,'[~ines.david] tu peux regarder quand tu as un moment ?','2026-02-02 08:07:11.000','2026-02-02 08:07:11.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10181,10088,7,'[~jeanne.dupont] tu peux regarder quand tu as un moment ?','2026-01-11 13:18:26.000','2026-01-11 13:18:26.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10182,10088,9,'Je confirme, reproduit sur l\'environnement de recette.','2026-01-16 14:42:26.000','2026-01-16 14:42:26.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10183,10088,9,'Point fait avec le client ce matin, il valide la solution proposée.','2026-01-26 12:43:06.000','2026-01-26 12:43:06.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10184,10088,20,'C\'est bon pour moi, on peut fermer.','2026-02-10 18:40:58.000','2026-02-10 18:40:58.000',20,NULL,NULL);
INSERT INTO `comments` VALUES (10185,10092,4,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-01-12 15:23:32.000','2026-01-12 15:23:32.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10186,10093,18,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-01-20 17:46:08.000','2026-01-20 17:46:08.000',18,NULL,NULL);
INSERT INTO `comments` VALUES (10187,10093,5,'[~julien.roux] tu peux regarder quand tu as un moment ?','2026-01-23 07:39:03.000','2026-01-23 07:39:03.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10188,10093,2,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-01-30 11:07:48.000','2026-01-30 11:07:48.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10189,10093,9,'Même cause que DEMO-25, je fais le lien.','2026-02-10 13:43:12.000','2026-02-10 13:43:12.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10190,10095,10,'Je confirme, reproduit sur l\'environnement de recette.','2026-02-04 18:25:47.000','2026-02-04 18:25:47.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10191,10095,8,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-02-17 15:26:18.000','2026-02-17 15:26:18.000',8,NULL,NULL);
INSERT INTO `comments` VALUES (10192,10095,8,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-02-17 16:33:58.000','2026-02-17 16:33:58.000',8,NULL,NULL);
INSERT INTO `comments` VALUES (10193,10097,14,'Point fait avec le client ce matin, il valide la solution proposée.','2026-01-31 12:31:40.000','2026-01-31 12:31:40.000',14,NULL,NULL);
INSERT INTO `comments` VALUES (10194,10097,14,'Je confirme, reproduit sur l\'environnement de recette.','2026-02-12 15:19:41.000','2026-02-13 18:14:53.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10195,10097,8,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-02-17 10:15:50.000','2026-02-17 10:15:50.000',8,NULL,NULL);
INSERT INTO `comments` VALUES (10196,10097,10,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-02-19 10:52:01.000','2026-02-19 10:52:01.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10197,10098,10,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-01-30 07:48:50.000','2026-01-30 07:48:50.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10198,10100,15,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-02-03 09:05:39.000','2026-02-03 09:05:39.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10199,10100,15,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-02-13 12:47:25.000','2026-02-13 12:47:25.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10200,10101,16,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-01-31 14:19:33.000','2026-01-31 14:19:33.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10201,10101,19,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-01-31 17:37:34.000','2026-01-31 17:37:34.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10202,10101,18,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-02-04 09:32:00.000','2026-02-04 09:32:00.000',18,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10203,10101,2,'C\'est bon pour moi, on peut fermer.','2026-02-08 14:49:37.000','2026-02-08 14:49:37.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10204,10101,6,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-02-12 13:20:20.000','2026-02-12 13:20:20.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10205,10101,19,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-02-25 16:05:51.000','2026-02-25 16:05:51.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10206,10103,5,'C\'est bon pour moi, on peut fermer.','2026-02-04 10:21:44.000','2026-02-04 10:21:44.000',5,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10207,10104,16,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-02-12 09:36:04.000','2026-02-12 09:36:04.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10208,10104,10,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-02-27 11:03:11.000','2026-02-28 10:36:25.000',1,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10209,10105,11,'Point fait avec le client ce matin, il valide la solution proposée.','2026-02-14 18:38:00.000','2026-02-14 18:38:00.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10210,10105,12,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-02-19 10:15:14.000','2026-02-19 10:15:14.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10211,10105,20,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-02-27 09:57:38.000','2026-02-27 09:57:38.000',20,NULL,NULL);
INSERT INTO `comments` VALUES (10212,10105,5,'Point fait avec le client ce matin, il valide la solution proposée.','2026-03-09 15:27:25.000','2026-03-09 15:27:25.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10213,10107,7,'Je confirme, reproduit sur l\'environnement de recette.','2026-02-12 07:55:18.000','2026-02-12 07:55:18.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10214,10107,4,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-02-12 11:36:54.000','2026-02-12 11:36:54.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10215,10107,11,'Même cause que WEB-6, je fais le lien.','2026-02-22 10:27:20.000','2026-02-22 10:27:20.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10216,10107,10,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-02-28 15:46:59.000','2026-02-28 15:46:59.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10217,10107,2,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-03-05 16:59:57.000','2026-03-05 16:59:57.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10218,10107,20,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-03-09 15:09:46.000','2026-03-09 15:09:46.000',20,NULL,NULL);
INSERT INTO `comments` VALUES (10219,10109,19,'Point fait avec le client ce matin, il valide la solution proposée.','2026-02-20 11:31:43.000','2026-02-20 11:31:43.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10220,10109,11,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-03-04 11:58:33.000','2026-03-04 11:58:33.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10221,10109,7,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-03-09 12:37:57.000','2026-03-11 10:55:12.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10222,10109,17,'Même cause que DEMO-24, je fais le lien.','2026-03-20 10:29:23.000','2026-03-20 10:29:23.000',17,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10223,10110,5,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-02-16 13:50:22.000','2026-02-16 13:50:22.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10224,10113,2,'Je confirme, reproduit sur l\'environnement de recette.','2026-02-22 10:42:47.000','2026-02-24 11:22:13.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10225,10113,15,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-02-27 18:45:48.000','2026-02-27 18:45:48.000',15,NULL,10002);
INSERT INTO `comments` VALUES (10226,10118,15,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-03-01 09:35:56.000','2026-03-01 09:35:56.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10227,10118,22,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-03-12 10:47:40.000','2026-03-12 10:47:40.000',22,NULL,NULL);
INSERT INTO `comments` VALUES (10228,10118,7,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-03-16 08:29:50.000','2026-03-16 08:29:50.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10229,10118,21,'Même cause que OPS-12, je fais le lien.','2026-03-26 16:29:10.000','2026-03-26 16:29:10.000',21,NULL,NULL);
INSERT INTO `comments` VALUES (10230,10118,21,'[~maxime.bertrand] tu peux regarder quand tu as un moment ?','2026-03-28 17:38:31.000','2026-03-28 17:38:31.000',21,NULL,NULL);
INSERT INTO `comments` VALUES (10231,10118,12,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-03-31 12:53:52.000','2026-03-31 12:53:52.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10232,10119,15,'Même cause que DEMO-50, je fais le lien.','2026-02-27 12:10:46.000','2026-02-27 12:10:46.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10233,10119,15,'Même cause que DEMO-50, je fais le lien.','2026-03-11 11:46:44.000','2026-03-11 11:46:44.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10234,10121,12,'Même cause que OPS-20, je fais le lien.','2026-03-01 07:49:39.000','2026-03-01 07:49:39.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10235,10121,12,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-03-08 09:11:18.000','2026-03-08 09:11:18.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10236,10122,5,'[~camille.petit] tu peux regarder quand tu as un moment ?','2026-03-03 09:30:29.000','2026-03-03 09:30:29.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10237,10122,5,'[~thomas.moreau] tu peux regarder quand tu as un moment ?','2026-03-04 10:21:00.000','2026-03-04 10:21:00.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10238,10122,5,'Même cause que WEB-16, je fais le lien.','2026-03-10 16:51:22.000','2026-03-10 16:51:22.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10239,10123,21,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-02-28 08:20:39.000','2026-02-28 08:20:39.000',21,NULL,NULL);
INSERT INTO `comments` VALUES (10240,10126,19,'Point fait avec le client ce matin, il valide la solution proposée.','2026-02-24 18:46:34.000','2026-02-24 18:46:34.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10241,10127,10,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-03-05 08:30:15.000','2026-03-05 08:30:15.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10242,10127,9,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-03-20 13:23:11.000','2026-03-20 13:23:11.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10243,10127,9,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-03-28 10:55:37.000','2026-03-28 10:55:37.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10244,10128,19,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-02-28 16:55:30.000','2026-03-03 17:08:18.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10245,10128,3,'C\'est bon pour moi, on peut fermer.','2026-03-14 13:35:09.000','2026-03-14 13:35:09.000',3,NULL,NULL);
INSERT INTO `comments` VALUES (10246,10128,10,'Je confirme, reproduit sur l\'environnement de recette.','2026-03-24 15:56:05.000','2026-03-24 15:56:05.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10247,10128,18,'C\'est bon pour moi, on peut fermer.','2026-04-08 11:39:25.000','2026-04-08 11:39:25.000',18,NULL,NULL);
INSERT INTO `comments` VALUES (10248,10128,2,'C\'est bon pour moi, on peut fermer.','2026-04-09 11:00:38.000','2026-04-09 11:00:38.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10249,10128,2,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-04-12 16:36:15.000','2026-04-12 16:36:15.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10250,10129,12,'C\'est bon pour moi, on peut fermer.','2026-03-01 18:11:39.000','2026-03-04 07:43:09.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10251,10129,10,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-03-16 08:02:50.000','2026-03-16 08:02:50.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10252,10131,3,'Même cause que WEB-34, je fais le lien.','2026-03-12 14:48:49.000','2026-03-12 14:48:49.000',3,NULL,NULL);
INSERT INTO `comments` VALUES (10253,10131,5,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-03-13 09:47:38.000','2026-03-13 09:47:38.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10254,10131,5,'Même cause que WEB-32, je fais le lien.','2026-03-23 07:35:48.000','2026-03-23 07:35:48.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10255,10131,20,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-04-05 14:23:59.000','2026-04-05 14:23:59.000',20,NULL,NULL);
INSERT INTO `comments` VALUES (10256,10131,11,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-04-10 06:43:37.000','2026-04-10 06:43:37.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10257,10132,2,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-03-21 07:15:21.000','2026-03-21 07:15:21.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10258,10132,7,'C\'est bon pour moi, on peut fermer.','2026-03-25 17:31:15.000','2026-03-27 11:14:54.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10259,10132,11,'C\'est bon pour moi, on peut fermer.','2026-04-06 13:05:35.000','2026-04-06 13:05:35.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10260,10133,13,'Point fait avec le client ce matin, il valide la solution proposée.','2026-03-21 09:07:28.000','2026-03-21 09:07:28.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10261,10133,15,'Même cause que DEMO-10, je fais le lien.','2026-03-28 12:22:38.000','2026-03-28 12:22:38.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10262,10133,3,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-04-01 06:24:30.000','2026-04-01 10:11:25.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10263,10133,2,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-04-13 07:59:04.000','2026-04-13 07:59:04.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10264,10134,7,'[~antoine.lefebvre] tu peux regarder quand tu as un moment ?','2026-03-15 10:00:25.000','2026-03-15 10:00:25.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10265,10134,12,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-03-17 16:59:24.000','2026-03-17 16:59:24.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10266,10134,16,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-03-22 15:32:36.000','2026-03-22 15:32:36.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10267,10136,17,'Point fait avec le client ce matin, il valide la solution proposée.','2026-03-19 09:03:19.000','2026-03-19 09:03:19.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10268,10136,6,'C\'est bon pour moi, on peut fermer.','2026-03-27 11:05:02.000','2026-03-27 11:05:02.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10269,10139,16,'Je confirme, reproduit sur l\'environnement de recette.','2026-03-20 13:53:47.000','2026-03-20 13:53:47.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10270,10139,7,'Point fait avec le client ce matin, il valide la solution proposée.','2026-04-04 17:48:37.000','2026-04-04 17:48:37.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10271,10140,2,'C\'est bon pour moi, on peut fermer.','2026-03-25 14:07:27.000','2026-03-25 14:07:27.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10272,10140,16,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-03-31 09:24:45.000','2026-03-31 09:24:45.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10273,10141,11,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-03-22 15:15:52.000','2026-03-22 15:15:52.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10274,10141,4,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-04-06 15:12:14.000','2026-04-09 12:28:39.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10275,10141,6,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-04-15 09:22:52.000','2026-04-15 09:22:52.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10276,10142,7,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-03-28 17:40:41.000','2026-03-28 17:40:41.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10277,10142,9,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-04-02 09:56:44.000','2026-04-04 09:49:34.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10278,10142,7,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-04-06 15:06:12.000','2026-04-06 15:06:12.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10279,10142,7,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-04-17 17:51:36.000','2026-04-17 17:51:36.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10280,10143,23,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-03-29 10:34:59.000','2026-03-29 10:34:59.000',23,NULL,NULL);
INSERT INTO `comments` VALUES (10281,10143,15,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-04-10 06:14:40.000','2026-04-10 06:14:40.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10282,10144,15,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-04-04 06:25:35.000','2026-04-04 06:25:35.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10283,10145,6,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-03-24 13:39:38.000','2026-03-24 13:39:38.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10284,10145,10,'Même cause que WEB-9, je fais le lien.','2026-03-25 17:28:37.000','2026-03-25 17:28:37.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10285,10145,6,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-04-08 10:01:19.000','2026-04-08 10:01:19.000',6,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10286,10145,6,'Point fait avec le client ce matin, il valide la solution proposée.','2026-04-18 12:55:03.000','2026-04-18 12:55:03.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10287,10145,7,'Je confirme, reproduit sur l\'environnement de recette.','2026-04-29 13:08:32.000','2026-04-29 13:08:32.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10288,10145,12,'Même cause que WEB-37, je fais le lien.','2026-05-03 14:36:16.000','2026-05-03 14:36:16.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10289,10146,20,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-03-26 17:08:09.000','2026-03-26 17:08:09.000',20,NULL,NULL);
INSERT INTO `comments` VALUES (10290,10147,18,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-04-06 16:23:20.000','2026-04-07 12:35:24.000',18,NULL,NULL);
INSERT INTO `comments` VALUES (10291,10148,23,'C\'est bon pour moi, on peut fermer.','2026-04-09 12:31:52.000','2026-04-09 12:31:52.000',23,NULL,NULL);
INSERT INTO `comments` VALUES (10292,10148,11,'Même cause que OPS-22, je fais le lien.','2026-04-13 13:54:01.000','2026-04-13 13:54:01.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10293,10151,10,'[~maxime.bertrand] tu peux regarder quand tu as un moment ?','2026-04-14 17:46:26.000','2026-04-14 17:47:26.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10294,10151,8,'Point fait avec le client ce matin, il valide la solution proposée.','2026-04-14 17:53:30.000','2026-04-14 17:53:30.000',8,NULL,NULL);
INSERT INTO `comments` VALUES (10295,10151,14,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-04-28 17:36:15.000','2026-04-30 14:57:19.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10296,10152,4,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-04-16 07:01:41.000','2026-04-16 07:01:41.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10297,10152,13,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-04-29 12:26:35.000','2026-04-29 12:26:35.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10298,10152,17,'C\'est bon pour moi, on peut fermer.','2026-05-01 16:29:05.000','2026-05-01 16:29:05.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10299,10153,17,'[~ines.david] tu peux regarder quand tu as un moment ?','2026-04-22 15:13:29.000','2026-04-22 15:13:29.000',17,NULL,10002);
INSERT INTO `comments` VALUES (10300,10153,15,'Point fait avec le client ce matin, il valide la solution proposée.','2026-04-22 17:59:01.000','2026-04-22 17:59:01.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10301,10153,17,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-05-01 14:02:17.000','2026-05-01 14:02:17.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10302,10154,16,'C\'est bon pour moi, on peut fermer.','2026-04-14 17:58:16.000','2026-04-14 17:58:16.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10303,10154,17,'Je confirme, reproduit sur l\'environnement de recette.','2026-04-17 08:38:28.000','2026-04-17 08:38:28.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10304,10154,17,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-04-18 09:31:54.000','2026-04-18 09:31:54.000',17,NULL,10002);
INSERT INTO `comments` VALUES (10305,10156,10,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-04-21 11:14:05.000','2026-04-21 11:14:05.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10306,10156,6,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-04-28 17:23:08.000','2026-04-28 17:23:08.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10307,10157,8,'[~paul.martin] tu peux regarder quand tu as un moment ?','2026-04-19 08:38:04.000','2026-04-19 08:38:04.000',8,NULL,NULL);
INSERT INTO `comments` VALUES (10308,10158,6,'C\'est bon pour moi, on peut fermer.','2026-04-19 07:05:28.000','2026-04-21 11:37:33.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10309,10159,12,'[~maxime.bertrand] tu peux regarder quand tu as un moment ?','2026-04-22 14:07:13.000','2026-04-22 14:07:13.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10310,10159,19,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-04-22 14:11:04.000','2026-04-22 14:11:04.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10311,10160,11,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-04-24 10:05:56.000','2026-04-24 10:05:56.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10312,10161,16,'Je confirme, reproduit sur l\'environnement de recette.','2026-04-19 10:10:27.000','2026-04-19 10:10:27.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10313,10161,3,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-05-02 10:09:49.000','2026-05-02 10:09:49.000',3,NULL,NULL);
INSERT INTO `comments` VALUES (10314,10161,2,'Je confirme, reproduit sur l\'environnement de recette.','2026-05-11 11:32:00.000','2026-05-11 11:32:00.000',2,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10315,10162,9,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-04-17 15:56:55.000','2026-04-17 15:56:55.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10316,10162,12,'[~antoine.lefebvre] tu peux regarder quand tu as un moment ?','2026-04-29 13:53:14.000','2026-04-29 13:53:14.000',12,NULL,10002);
INSERT INTO `comments` VALUES (10317,10164,4,'Je confirme, reproduit sur l\'environnement de recette.','2026-04-22 12:37:32.000','2026-04-22 12:37:32.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10318,10164,5,'[~paul.martin] tu peux regarder quand tu as un moment ?','2026-05-03 15:48:43.000','2026-05-03 15:48:43.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10319,10166,15,'C\'est bon pour moi, on peut fermer.','2026-04-28 10:42:34.000','2026-04-28 10:42:34.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10320,10167,5,'[~nicolas.laurent] tu peux regarder quand tu as un moment ?','2026-05-02 16:33:10.000','2026-05-02 16:33:10.000',5,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10321,10167,17,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-05-02 16:37:47.000','2026-05-02 16:37:47.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10322,10168,9,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-04-24 12:36:25.000','2026-04-24 12:36:25.000',9,NULL,10002);
INSERT INTO `comments` VALUES (10323,10168,9,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-05-03 15:02:22.000','2026-05-03 15:02:22.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10324,10168,5,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-05-06 09:43:42.000','2026-05-08 11:20:57.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10325,10169,11,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-05-03 14:53:29.000','2026-05-03 14:53:29.000',11,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10326,10169,10,'[~lucas.bernard] tu peux regarder quand tu as un moment ?','2026-05-06 08:22:28.000','2026-05-06 08:22:28.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10327,10170,6,'[~paul.martin] tu peux regarder quand tu as un moment ?','2026-05-07 17:04:32.000','2026-05-08 07:33:17.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10328,10170,9,'[~ines.david] tu peux regarder quand tu as un moment ?','2026-05-16 07:06:36.000','2026-05-16 07:06:36.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10329,10170,10,'Je confirme, reproduit sur l\'environnement de recette.','2026-05-20 10:58:02.000','2026-05-20 10:58:02.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10330,10170,18,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-05-23 08:54:48.000','2026-05-23 08:54:48.000',18,NULL,NULL);
INSERT INTO `comments` VALUES (10331,10171,14,'[~maxime.bertrand] tu peux regarder quand tu as un moment ?','2026-05-06 07:59:39.000','2026-05-06 07:59:39.000',14,NULL,NULL);
INSERT INTO `comments` VALUES (10332,10172,6,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-04-29 17:42:42.000','2026-04-29 17:42:42.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10333,10172,6,'C\'est bon pour moi, on peut fermer.','2026-05-13 16:21:02.000','2026-05-13 16:21:02.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10334,10174,7,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-05-06 15:22:12.000','2026-05-07 14:46:05.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10335,10174,6,'[~antoine.lefebvre] tu peux regarder quand tu as un moment ?','2026-05-11 07:49:57.000','2026-05-11 12:12:35.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10336,10174,20,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-05-24 10:37:08.000','2026-05-24 10:37:08.000',20,NULL,NULL);
INSERT INTO `comments` VALUES (10337,10175,12,'Point fait avec le client ce matin, il valide la solution proposée.','2026-05-11 17:47:34.000','2026-05-11 17:47:34.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10338,10175,22,'Même cause que OPS-16, je fais le lien.','2026-05-24 07:08:33.000','2026-05-24 07:08:33.000',22,NULL,NULL);
INSERT INTO `comments` VALUES (10339,10175,7,'Je confirme, reproduit sur l\'environnement de recette.','2026-06-01 15:19:45.000','2026-06-01 15:19:45.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10340,10175,19,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-06-07 16:13:52.000','2026-06-07 16:13:52.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10341,10175,9,'Je confirme, reproduit sur l\'environnement de recette.','2026-06-20 10:05:42.000','2026-06-20 10:05:42.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10342,10175,2,'[~julien.roux] tu peux regarder quand tu as un moment ?','2026-06-25 08:03:02.000','2026-06-25 08:03:02.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10343,10176,12,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-05-07 14:37:18.000','2026-05-07 14:37:18.000',12,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10344,10176,17,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-05-20 13:57:21.000','2026-05-22 07:07:55.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10345,10176,4,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-05-31 14:43:29.000','2026-05-31 14:43:29.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10346,10176,5,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-06-03 15:17:25.000','2026-06-03 15:17:25.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10347,10177,7,'Même cause que OPS-22, je fais le lien.','2026-05-17 09:14:51.000','2026-05-17 09:14:51.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10348,10178,11,'Même cause que WEB-21, je fais le lien.','2026-05-18 09:16:40.000','2026-05-18 09:16:40.000',11,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10349,10178,4,'Je confirme, reproduit sur l\'environnement de recette.','2026-05-25 10:28:31.000','2026-05-25 10:28:31.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10350,10178,18,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-06-05 17:41:49.000','2026-06-05 17:41:49.000',18,NULL,NULL);
INSERT INTO `comments` VALUES (10351,10178,20,'C\'est bon pour moi, on peut fermer.','2026-06-07 07:06:49.000','2026-06-07 07:06:49.000',20,NULL,NULL);
INSERT INTO `comments` VALUES (10352,10179,7,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-05-15 11:56:25.000','2026-05-15 11:56:25.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10353,10180,8,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-05-12 09:19:44.000','2026-05-12 11:06:22.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10354,10180,1,'Point fait avec le client ce matin, il valide la solution proposée.','2026-05-20 08:15:20.000','2026-05-20 08:15:20.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10355,10181,13,'Je confirme, reproduit sur l\'environnement de recette.','2026-05-16 13:19:47.000','2026-05-16 13:19:47.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10356,10181,15,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-05-31 16:38:20.000','2026-05-31 16:38:20.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10357,10181,20,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-06-02 10:37:30.000','2026-06-02 14:58:19.000',20,NULL,NULL);
INSERT INTO `comments` VALUES (10358,10181,5,'Je confirme, reproduit sur l\'environnement de recette.','2026-06-08 09:10:13.000','2026-06-08 09:10:13.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10359,10182,5,'C\'est bon pour moi, on peut fermer.','2026-05-16 17:51:57.000','2026-05-16 17:51:57.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10360,10182,4,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-05-20 15:55:45.000','2026-05-21 16:06:11.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10361,10185,5,'Point fait avec le client ce matin, il valide la solution proposée.','2026-05-22 09:27:03.000','2026-05-22 09:27:03.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10362,10185,12,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-05-27 09:40:14.000','2026-05-27 09:40:14.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10363,10185,2,'Même cause que WEB-51, je fais le lien.','2026-05-27 12:56:40.000','2026-05-27 12:56:40.000',2,NULL,10002);
INSERT INTO `comments` VALUES (10364,10185,4,'Même cause que WEB-19, je fais le lien.','2026-06-06 09:19:53.000','2026-06-06 09:19:53.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10365,10187,11,'[~paul.martin] tu peux regarder quand tu as un moment ?','2026-05-23 16:03:12.000','2026-05-23 16:03:12.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10366,10187,22,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-05-23 17:16:33.000','2026-05-26 16:37:37.000',22,NULL,NULL);
INSERT INTO `comments` VALUES (10367,10187,7,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-06-04 11:56:00.000','2026-06-04 11:56:00.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10368,10188,15,'Même cause que DEMO-67, je fais le lien.','2026-05-29 15:51:45.000','2026-05-29 15:51:45.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10369,10189,20,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-05-23 12:53:24.000','2026-05-23 12:53:24.000',20,NULL,NULL);
INSERT INTO `comments` VALUES (10370,10190,3,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-06-01 12:51:38.000','2026-06-01 12:51:38.000',3,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10371,10190,6,'Même cause que DEMO-59, je fais le lien.','2026-06-05 13:49:28.000','2026-06-05 13:49:28.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10372,10190,3,'Je confirme, reproduit sur l\'environnement de recette.','2026-06-12 06:10:01.000','2026-06-12 06:10:01.000',3,NULL,NULL);
INSERT INTO `comments` VALUES (10373,10190,6,'Même cause que DEMO-54, je fais le lien.','2026-06-23 11:03:27.000','2026-06-23 11:03:27.000',6,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10374,10190,2,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-06-26 13:11:45.000','2026-06-26 13:11:45.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10375,10192,19,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-06-01 11:05:15.000','2026-06-01 11:05:15.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10376,10192,13,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-06-13 12:50:38.000','2026-06-13 12:50:38.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10377,10192,17,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-06-14 12:02:09.000','2026-06-14 12:02:09.000',17,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10378,10194,1,'Même cause que RH-8, je fais le lien.','2026-05-31 13:25:14.000','2026-05-31 13:25:14.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10379,10194,1,'C\'est bon pour moi, on peut fermer.','2026-06-09 08:51:57.000','2026-06-09 08:51:57.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10380,10194,14,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-06-18 10:13:29.000','2026-06-18 10:13:29.000',14,NULL,NULL);
INSERT INTO `comments` VALUES (10381,10195,4,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-05-23 08:43:34.000','2026-05-23 08:43:34.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10382,10195,5,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-06-05 07:40:48.000','2026-06-05 07:40:48.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10383,10195,6,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-06-20 06:30:22.000','2026-06-20 06:30:22.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10384,10196,17,'Je confirme, reproduit sur l\'environnement de recette.','2026-05-23 11:32:00.000','2026-05-23 11:32:00.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10385,10196,12,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-06-01 15:33:55.000','2026-06-01 15:33:55.000',12,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10386,10196,13,'Point fait avec le client ce matin, il valide la solution proposée.','2026-06-14 13:20:44.000','2026-06-14 13:20:44.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10387,10197,14,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-06-06 06:13:33.000','2026-06-06 06:13:33.000',14,NULL,NULL);
INSERT INTO `comments` VALUES (10388,10197,10,'Je confirme, reproduit sur l\'environnement de recette.','2026-06-09 06:53:47.000','2026-06-09 06:53:47.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10389,10198,10,'Même cause que WEB-39, je fais le lien.','2026-06-08 09:14:25.000','2026-06-08 15:41:05.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10390,10199,16,'Point fait avec le client ce matin, il valide la solution proposée.','2026-05-29 12:31:15.000','2026-05-29 12:31:15.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10391,10199,11,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-06-06 16:06:22.000','2026-06-06 16:06:22.000',11,NULL,10002);
INSERT INTO `comments` VALUES (10392,10199,6,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-06-12 10:25:33.000','2026-06-12 10:25:33.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10393,10199,11,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-06-15 12:47:09.000','2026-06-15 12:47:09.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10394,10199,18,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-06-21 16:27:34.000','2026-06-21 16:27:34.000',18,NULL,NULL);
INSERT INTO `comments` VALUES (10395,10200,2,'Je confirme, reproduit sur l\'environnement de recette.','2026-05-30 12:24:50.000','2026-05-30 12:24:50.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10396,10200,2,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-06-12 14:19:18.000','2026-06-12 14:19:18.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10397,10201,11,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-05-29 11:07:47.000','2026-05-29 11:07:47.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10398,10201,6,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-05-31 13:09:06.000','2026-05-31 13:09:06.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10399,10202,12,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-05-29 06:10:54.000','2026-05-29 06:10:54.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10400,10202,18,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-06-01 14:50:24.000','2026-06-02 16:08:34.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10401,10202,18,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-06-12 12:28:13.000','2026-06-14 10:24:39.000',18,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10402,10203,10,'C\'est bon pour moi, on peut fermer.','2026-06-01 10:05:10.000','2026-06-01 10:05:10.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10403,10203,10,'Point fait avec le client ce matin, il valide la solution proposée.','2026-06-04 06:48:41.000','2026-06-04 06:48:41.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10404,10204,11,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-06-09 13:58:42.000','2026-06-09 13:58:42.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10405,10205,15,'Point fait avec le client ce matin, il valide la solution proposée.','2026-06-06 10:51:34.000','2026-06-06 10:51:34.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10406,10205,22,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-06-17 09:54:20.000','2026-06-17 09:54:20.000',22,NULL,NULL);
INSERT INTO `comments` VALUES (10407,10207,12,'C\'est bon pour moi, on peut fermer.','2026-06-01 14:13:12.000','2026-06-01 14:13:12.000',12,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10408,10207,16,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-06-11 10:03:02.000','2026-06-11 10:03:02.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10409,10207,13,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-06-16 14:55:44.000','2026-06-16 14:55:44.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10410,10207,17,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-06-30 10:44:02.000','2026-06-30 10:44:02.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10411,10207,17,'Même cause que WEB-53, je fais le lien.','2026-07-04 07:24:28.000','2026-07-04 07:24:28.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10412,10208,17,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-06-10 08:40:34.000','2026-06-10 08:40:34.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10413,10208,20,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-06-19 15:23:12.000','2026-06-19 15:23:12.000',20,NULL,NULL);
INSERT INTO `comments` VALUES (10414,10209,21,'[~hugo.robert] tu peux regarder quand tu as un moment ?','2026-06-09 12:35:51.000','2026-06-09 12:35:51.000',21,NULL,NULL);
INSERT INTO `comments` VALUES (10415,10211,7,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-06-20 07:49:25.000','2026-06-21 09:30:52.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10416,10214,17,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-06-23 09:20:20.000','2026-06-23 09:20:20.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10417,10214,2,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-06-26 17:49:20.000','2026-06-26 17:49:20.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10418,10214,2,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-06-29 15:01:53.000','2026-06-29 15:01:53.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10419,10215,17,'Point fait avec le client ce matin, il valide la solution proposée.','2026-06-18 07:34:15.000','2026-06-18 07:34:15.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10420,10216,16,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-06-29 14:43:34.000','2026-06-29 14:43:34.000',16,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10421,10216,20,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-07-13 09:49:58.000','2026-07-13 09:49:58.000',20,NULL,NULL);
INSERT INTO `comments` VALUES (10422,10216,11,'C\'est bon pour moi, on peut fermer.','2026-07-19 11:37:51.000','2026-07-19 11:37:51.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10423,10216,6,'[~antoine.lefebvre] tu peux regarder quand tu as un moment ?','2026-07-19 13:01:12.000','2026-07-19 13:01:12.000',6,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10424,10216,13,'[~julien.roux] tu peux regarder quand tu as un moment ?','2026-07-24 06:58:20.000','2026-07-24 06:58:20.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10425,10216,16,'[~camille.petit] tu peux regarder quand tu as un moment ?','2026-07-24 11:59:37.000','2026-07-24 11:59:37.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10426,10217,17,'C\'est bon pour moi, on peut fermer.','2026-06-26 08:10:20.000','2026-06-26 08:10:20.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10427,10217,6,'Je confirme, reproduit sur l\'environnement de recette.','2026-07-03 12:49:20.000','2026-07-03 12:49:20.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10428,10217,17,'Même cause que DEMO-14, je fais le lien.','2026-07-09 07:32:20.000','2026-07-09 07:32:20.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10429,10217,4,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-07-18 06:42:58.000','2026-07-18 06:42:58.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10430,10217,17,'Je confirme, reproduit sur l\'environnement de recette.','2026-07-20 17:28:03.000','2026-07-21 11:27:04.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10431,10218,9,'Même cause que WEB-39, je fais le lien.','2026-06-30 15:30:52.000','2026-06-30 15:30:52.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10432,10219,2,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-06-19 15:53:37.000','2026-06-19 15:53:37.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10433,10219,19,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-06-21 16:21:50.000','2026-06-21 16:21:50.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10434,10219,15,'Même cause que DEMO-17, je fais le lien.','2026-07-03 10:06:07.000','2026-07-03 10:06:07.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10435,10219,15,'[~paul.martin] tu peux regarder quand tu as un moment ?','2026-07-06 09:32:24.000','2026-07-06 09:32:24.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10436,10220,7,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-06-24 10:01:02.000','2026-06-24 10:01:02.000',7,NULL,10002);
INSERT INTO `comments` VALUES (10437,10223,19,'Même cause que WEB-2, je fais le lien.','2026-06-27 09:03:02.000','2026-06-27 09:03:02.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10438,10223,2,'C\'est bon pour moi, on peut fermer.','2026-07-11 14:04:12.000','2026-07-11 14:04:12.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10439,10223,2,'Je confirme, reproduit sur l\'environnement de recette.','2026-07-16 17:08:14.000','2026-07-16 17:08:14.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10440,10223,13,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-07-27 08:03:34.000','2026-07-27 08:03:34.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10441,10224,21,'Même cause que OPS-22, je fais le lien.','2026-07-02 17:49:29.000','2026-07-02 17:49:29.000',21,NULL,NULL);
INSERT INTO `comments` VALUES (10442,10224,7,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-07-10 15:36:15.000','2026-07-10 15:36:15.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10443,10226,12,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-06-27 06:01:37.000','2026-06-27 06:01:37.000',12,NULL,10002);
INSERT INTO `comments` VALUES (10444,10226,9,'[~nicolas.laurent] tu peux regarder quand tu as un moment ?','2026-06-28 13:37:14.000','2026-06-28 15:25:49.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10445,10226,11,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-07-06 13:36:18.000','2026-07-06 13:36:18.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10446,10226,7,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-07-06 15:11:53.000','2026-07-06 15:11:53.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10447,10228,16,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-07-04 09:03:38.000','2026-07-04 09:03:38.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10448,10228,9,'Même cause que WEB-23, je fais le lien.','2026-07-14 08:16:13.000','2026-07-14 08:16:13.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10449,10229,5,'Je confirme, reproduit sur l\'environnement de recette.','2026-07-14 09:01:35.000','2026-07-14 09:01:35.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10450,10229,7,'[~ines.david] tu peux regarder quand tu as un moment ?','2026-07-18 15:29:34.000','2026-07-18 15:29:34.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10451,10230,17,'Même cause que WEB-26, je fais le lien.','2026-07-07 17:13:41.000','2026-07-07 17:13:41.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10452,10230,3,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-07-11 17:42:55.000','2026-07-11 17:42:55.000',3,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10453,10230,11,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-07-18 12:25:10.000','2026-07-18 12:25:10.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10454,10231,2,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-07-15 13:50:11.000','2026-07-15 13:50:11.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10455,10231,7,'Même cause que WEB-16, je fais le lien.','2026-07-25 06:41:00.000','2026-07-25 06:41:00.000',7,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10456,10231,18,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-08-05 07:48:30.000','2026-08-05 07:48:30.000',18,NULL,NULL);
INSERT INTO `comments` VALUES (10457,10231,15,'Je confirme, reproduit sur l\'environnement de recette.','2026-08-14 16:40:16.000','2026-08-16 07:03:25.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10458,10231,7,'Je confirme, reproduit sur l\'environnement de recette.','2026-08-17 15:28:18.000','2026-08-17 15:28:18.000',7,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10459,10231,10,'[~paul.martin] tu peux regarder quand tu as un moment ?','2026-08-31 10:52:36.000','2026-08-31 10:52:36.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10460,10233,12,'C\'est bon pour moi, on peut fermer.','2026-07-11 10:29:05.000','2026-07-11 10:29:05.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10461,10233,9,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-07-22 13:38:55.000','2026-07-22 13:38:55.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10462,10233,2,'Point fait avec le client ce matin, il valide la solution proposée.','2026-07-22 16:47:04.000','2026-07-22 16:47:04.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10463,10234,10,'Je confirme, reproduit sur l\'environnement de recette.','2026-07-17 06:13:27.000','2026-07-17 06:13:27.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10464,10234,10,'Je confirme, reproduit sur l\'environnement de recette.','2026-07-19 08:56:47.000','2026-07-19 08:56:47.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10465,10234,8,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-07-26 09:09:41.000','2026-07-26 09:09:41.000',8,NULL,NULL);
INSERT INTO `comments` VALUES (10466,10234,10,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-08-04 14:13:19.000','2026-08-04 14:13:19.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10467,10234,10,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-08-10 13:00:03.000','2026-08-10 13:00:03.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10468,10236,13,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-07-12 06:50:26.000','2026-07-12 06:50:26.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10469,10237,18,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-07-16 08:15:11.000','2026-07-16 08:15:11.000',18,NULL,NULL);
INSERT INTO `comments` VALUES (10470,10238,13,'Même cause que DEMO-80, je fais le lien.','2026-07-19 09:46:03.000','2026-07-19 09:46:03.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10471,10239,16,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-07-19 17:41:52.000','2026-07-19 17:41:52.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10472,10239,3,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-08-02 13:55:03.000','2026-08-02 13:55:03.000',3,NULL,10002);
INSERT INTO `comments` VALUES (10473,10241,5,'[~hugo.robert] tu peux regarder quand tu as un moment ?','2026-07-25 09:48:24.000','2026-07-25 09:48:24.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10474,10241,11,'Point fait avec le client ce matin, il valide la solution proposée.','2026-08-08 10:25:10.000','2026-08-08 10:25:10.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10475,10244,7,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-07-30 06:47:32.000','2026-07-30 06:47:32.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10476,10244,16,'Je confirme, reproduit sur l\'environnement de recette.','2026-08-04 14:53:40.000','2026-08-05 14:34:03.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10477,10244,7,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-08-17 07:21:39.000','2026-08-17 07:21:39.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10478,10244,4,'Point fait avec le client ce matin, il valide la solution proposée.','2026-08-29 11:33:15.000','2026-08-29 11:33:15.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10479,10245,10,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-07-21 15:59:53.000','2026-07-21 15:59:53.000',10,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10480,10245,2,'C\'est bon pour moi, on peut fermer.','2026-07-31 15:25:23.000','2026-07-31 15:25:23.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10481,10245,16,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-08-07 09:45:51.000','2026-08-07 09:45:51.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10482,10245,2,'[~nicolas.laurent] tu peux regarder quand tu as un moment ?','2026-08-10 11:44:07.000','2026-08-10 11:44:07.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10483,10245,2,'Je confirme, reproduit sur l\'environnement de recette.','2026-08-25 14:02:59.000','2026-08-25 14:02:59.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10484,10245,2,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-08-28 11:19:44.000','2026-08-28 11:19:44.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10485,10247,14,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-07-26 07:18:59.000','2026-07-26 07:18:59.000',14,NULL,NULL);
INSERT INTO `comments` VALUES (10486,10247,14,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-08-02 07:13:44.000','2026-08-02 07:13:44.000',14,NULL,NULL);
INSERT INTO `comments` VALUES (10487,10247,10,'C\'est bon pour moi, on peut fermer.','2026-08-05 06:06:54.000','2026-08-05 06:06:54.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10488,10247,8,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-08-05 11:05:12.000','2026-08-05 11:05:12.000',8,NULL,NULL);
INSERT INTO `comments` VALUES (10489,10247,14,'C\'est bon pour moi, on peut fermer.','2026-08-18 09:59:33.000','2026-08-18 09:59:33.000',14,NULL,NULL);
INSERT INTO `comments` VALUES (10490,10249,18,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-07-30 13:42:29.000','2026-07-30 13:42:29.000',18,NULL,NULL);
INSERT INTO `comments` VALUES (10491,10249,3,'Même cause que DEMO-43, je fais le lien.','2026-07-31 13:00:49.000','2026-07-31 13:00:49.000',3,NULL,NULL);
INSERT INTO `comments` VALUES (10492,10251,2,'Même cause que DEMO-8, je fais le lien.','2026-07-30 12:27:34.000','2026-07-30 12:27:34.000',2,NULL,10002);
INSERT INTO `comments` VALUES (10493,10251,2,'[~hugo.robert] tu peux regarder quand tu as un moment ?','2026-08-01 17:29:20.000','2026-08-01 17:29:20.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10494,10251,16,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-08-04 08:32:09.000','2026-08-04 08:32:09.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10495,10251,2,'Je confirme, reproduit sur l\'environnement de recette.','2026-08-10 12:57:15.000','2026-08-10 12:57:15.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10496,10252,13,'Point fait avec le client ce matin, il valide la solution proposée.','2026-08-14 11:49:23.000','2026-08-14 11:49:23.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10497,10253,17,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-07-31 17:33:31.000','2026-07-31 17:33:31.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10498,10253,9,'Je confirme, reproduit sur l\'environnement de recette.','2026-08-15 12:44:58.000','2026-08-15 12:44:58.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10499,10253,20,'Même cause que WEB-57, je fais le lien.','2026-08-16 08:34:24.000','2026-08-16 08:34:24.000',20,NULL,NULL);
INSERT INTO `comments` VALUES (10500,10254,17,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-08-12 12:57:17.000','2026-08-12 12:57:17.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10501,10254,13,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-08-20 10:52:22.000','2026-08-20 10:52:22.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10502,10254,16,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-08-24 13:25:49.000','2026-08-25 17:52:03.000',16,NULL,10002);
INSERT INTO `comments` VALUES (10503,10255,14,'Je confirme, reproduit sur l\'environnement de recette.','2026-08-08 09:43:09.000','2026-08-08 09:43:09.000',14,NULL,NULL);
INSERT INTO `comments` VALUES (10504,10256,5,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-08-18 13:27:07.000','2026-08-18 13:27:07.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10505,10256,5,'Même cause que WEB-61, je fais le lien.','2026-08-20 16:46:38.000','2026-08-20 16:46:38.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10506,10256,2,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-08-26 06:22:39.000','2026-08-26 06:22:39.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10507,10258,23,'[~antoine.lefebvre] tu peux regarder quand tu as un moment ?','2026-08-08 14:59:16.000','2026-08-08 14:59:16.000',23,NULL,NULL);
INSERT INTO `comments` VALUES (10508,10258,7,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-08-13 17:26:13.000','2026-08-13 17:26:13.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10509,10259,2,'Je confirme, reproduit sur l\'environnement de recette.','2026-08-21 10:21:24.000','2026-08-21 10:21:24.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10510,10259,9,'[~maxime.bertrand] tu peux regarder quand tu as un moment ?','2026-08-30 07:26:29.000','2026-08-30 07:26:29.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10511,10260,9,'Même cause que OPS-17, je fais le lien.','2026-08-23 12:49:52.000','2026-08-23 12:49:52.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10512,10260,9,'Je confirme, reproduit sur l\'environnement de recette.','2026-08-30 17:34:31.000','2026-08-30 17:34:31.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10513,10260,23,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-09-03 08:59:54.000','2026-09-03 08:59:54.000',23,NULL,NULL);
INSERT INTO `comments` VALUES (10514,10260,22,'Je confirme, reproduit sur l\'environnement de recette.','2026-09-18 16:26:57.000','2026-09-18 16:26:57.000',22,NULL,NULL);
INSERT INTO `comments` VALUES (10515,10261,23,'[~julien.roux] tu peux regarder quand tu as un moment ?','2026-08-21 06:34:56.000','2026-08-23 17:45:29.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10516,10261,7,'C\'est bon pour moi, on peut fermer.','2026-09-03 10:20:44.000','2026-09-03 10:20:44.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10517,10261,23,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-09-05 09:34:15.000','2026-09-05 09:34:15.000',23,NULL,NULL);
INSERT INTO `comments` VALUES (10518,10261,11,'Point fait avec le client ce matin, il valide la solution proposée.','2026-09-20 07:19:43.000','2026-09-20 07:19:43.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10519,10261,7,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-09-22 07:28:26.000','2026-09-22 07:28:26.000',7,NULL,10002);
INSERT INTO `comments` VALUES (10520,10262,19,'Même cause que OPS-13, je fais le lien.','2026-08-22 13:48:45.000','2026-08-22 13:48:45.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10521,10262,19,'Point fait avec le client ce matin, il valide la solution proposée.','2026-09-04 09:37:19.000','2026-09-04 09:37:19.000',19,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10522,10262,15,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-09-16 16:40:03.000','2026-09-16 16:40:03.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10523,10262,22,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-09-20 10:42:49.000','2026-09-20 10:42:49.000',22,NULL,NULL);
INSERT INTO `comments` VALUES (10524,10264,12,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-08-21 16:58:54.000','2026-08-21 16:58:54.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10525,10264,7,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-08-24 17:59:33.000','2026-08-24 17:59:33.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10526,10265,17,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-08-27 11:03:56.000','2026-08-27 11:03:56.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10527,10265,4,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-09-05 13:38:38.000','2026-09-05 15:44:57.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10528,10265,20,'[~jeanne.dupont] tu peux regarder quand tu as un moment ?','2026-09-13 12:36:06.000','2026-09-14 13:22:12.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10529,10266,2,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-09-01 17:43:56.000','2026-09-01 17:43:56.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10530,10266,5,'Point fait avec le client ce matin, il valide la solution proposée.','2026-09-04 09:07:07.000','2026-09-04 09:07:07.000',5,NULL,NULL);
INSERT INTO `comments` VALUES (10531,10266,6,'Je confirme, reproduit sur l\'environnement de recette.','2026-09-12 08:58:10.000','2026-09-12 08:58:10.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10532,10266,17,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-09-27 15:11:07.000','2026-09-29 13:41:30.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10533,10266,17,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-10-01 16:00:00.000','2026-10-01 16:00:00.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10534,10267,7,'C\'est bon pour moi, on peut fermer.','2026-08-26 16:54:30.000','2026-08-26 16:54:30.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10535,10267,21,'C\'est bon pour moi, on peut fermer.','2026-09-10 08:11:51.000','2026-09-13 12:51:32.000',21,NULL,NULL);
INSERT INTO `comments` VALUES (10536,10267,21,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-09-16 16:06:56.000','2026-09-16 16:06:56.000',21,NULL,NULL);
INSERT INTO `comments` VALUES (10537,10267,2,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-09-26 07:15:39.000','2026-09-26 07:15:39.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10538,10268,10,'C\'est bon pour moi, on peut fermer.','2026-09-03 17:56:33.000','2026-09-03 17:56:33.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10539,10268,10,'Je confirme, reproduit sur l\'environnement de recette.','2026-09-17 06:40:19.000','2026-09-17 06:40:19.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10540,10270,1,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-08-28 13:49:00.000','2026-08-28 13:49:00.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10541,10270,10,'Je confirme, reproduit sur l\'environnement de recette.','2026-09-09 13:40:27.000','2026-09-09 13:40:27.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10542,10270,10,'Même cause que RH-20, je fais le lien.','2026-09-13 15:39:09.000','2026-09-13 15:39:09.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10543,10271,20,'Même cause que DEMO-26, je fais le lien.','2026-09-06 08:59:02.000','2026-09-06 13:45:04.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10544,10271,19,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-09-16 17:14:42.000','2026-09-16 17:14:42.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10545,10271,16,'Je confirme, reproduit sur l\'environnement de recette.','2026-09-18 14:48:29.000','2026-09-18 14:48:29.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10546,10272,12,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-08-29 15:16:42.000','2026-08-29 15:16:42.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10547,10273,3,'C\'est bon pour moi, on peut fermer.','2026-08-31 12:45:19.000','2026-08-31 12:45:19.000',3,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10548,10273,6,'Point fait avec le client ce matin, il valide la solution proposée.','2026-09-06 15:45:07.000','2026-09-07 08:51:14.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10549,10275,11,'[~marie.durand] tu peux regarder quand tu as un moment ?','2026-09-06 08:48:31.000','2026-09-06 08:48:31.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10550,10275,19,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-09-06 16:20:30.000','2026-09-06 16:20:30.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10551,10275,23,'Même cause que OPS-8, je fais le lien.','2026-09-14 12:59:46.000','2026-09-14 12:59:46.000',23,NULL,NULL);
INSERT INTO `comments` VALUES (10552,10275,12,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-09-28 14:30:13.000','2026-09-28 14:30:13.000',12,NULL,10002);
INSERT INTO `comments` VALUES (10553,10275,19,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-09-30 08:39:54.000','2026-09-30 08:39:54.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10554,10275,7,'C\'est bon pour moi, on peut fermer.','2026-10-01 16:00:00.000','2026-10-01 16:00:00.000',7,NULL,NULL);
INSERT INTO `comments` VALUES (10555,10277,19,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-09-09 16:40:11.000','2026-09-09 16:40:11.000',19,NULL,10002);
INSERT INTO `comments` VALUES (10556,10278,8,'Même cause que RH-25, je fais le lien.','2026-09-02 08:10:31.000','2026-09-02 08:10:31.000',8,NULL,NULL);
INSERT INTO `comments` VALUES (10557,10278,8,'Point fait avec le client ce matin, il valide la solution proposée.','2026-09-09 07:45:32.000','2026-09-09 07:45:32.000',8,NULL,NULL);
INSERT INTO `comments` VALUES (10558,10278,8,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-09-16 07:39:30.000','2026-09-17 16:53:33.000',8,NULL,NULL);
INSERT INTO `comments` VALUES (10559,10279,19,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-09-12 10:30:05.000','2026-09-12 10:30:05.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10560,10280,18,'Point fait avec le client ce matin, il valide la solution proposée.','2026-09-06 16:46:46.000','2026-09-06 16:46:46.000',18,NULL,NULL);
INSERT INTO `comments` VALUES (10561,10281,16,'Je confirme, reproduit sur l\'environnement de recette.','2026-09-10 14:53:07.000','2026-09-10 14:53:07.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10562,10281,15,'[~maxime.bertrand] tu peux regarder quand tu as un moment ?','2026-09-14 08:37:26.000','2026-09-14 08:37:26.000',15,NULL,10002);
INSERT INTO `comments` VALUES (10563,10281,15,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-09-15 09:15:44.000','2026-09-15 09:15:44.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10564,10281,15,'Je confirme, reproduit sur l\'environnement de recette.','2026-09-22 11:16:08.000','2026-09-22 11:16:08.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10565,10281,16,'C\'est bon pour moi, on peut fermer.','2026-09-28 06:44:15.000','2026-09-29 15:39:26.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10566,10283,21,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-09-16 12:03:35.000','2026-09-16 12:03:35.000',21,NULL,NULL);
INSERT INTO `comments` VALUES (10567,10283,12,'C\'est bon pour moi, on peut fermer.','2026-09-29 17:58:26.000','2026-09-29 17:58:26.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10568,10283,23,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-10-01 16:00:00.000','2026-10-01 16:00:00.000',23,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10569,10283,21,'C\'est bon pour moi, on peut fermer.','2026-10-01 16:00:01.000','2026-10-01 16:00:02.000',21,NULL,NULL);
INSERT INTO `comments` VALUES (10570,10283,21,'Point fait avec le client ce matin, il valide la solution proposée.','2026-10-01 16:00:02.000','2026-10-01 16:00:02.000',21,NULL,NULL);
INSERT INTO `comments` VALUES (10571,10283,9,'Même cause que OPS-48, je fais le lien.','2026-10-01 16:00:03.000','2026-10-01 16:00:03.000',9,NULL,NULL);
INSERT INTO `comments` VALUES (10572,10284,10,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-09-13 09:02:20.000','2026-09-13 09:02:20.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10573,10284,15,'[~antoine.lefebvre] tu peux regarder quand tu as un moment ?','2026-09-21 12:18:25.000','2026-09-21 12:18:25.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10574,10285,15,'Point fait avec le client ce matin, il valide la solution proposée.','2026-09-17 16:51:25.000','2026-09-17 16:51:25.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10575,10286,6,'Même cause que WEB-18, je fais le lien.','2026-09-24 16:22:18.000','2026-09-24 16:22:18.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10576,10286,4,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-09-28 10:48:40.000','2026-09-28 10:48:40.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10577,10287,11,'Je confirme, reproduit sur l\'environnement de recette.','2026-09-13 14:03:46.000','2026-09-13 14:03:46.000',11,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10578,10288,4,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-09-15 07:42:55.000','2026-09-15 07:42:55.000',4,NULL,NULL);
INSERT INTO `comments` VALUES (10579,10288,12,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-09-30 16:18:17.000','2026-09-30 16:18:17.000',12,NULL,NULL);
INSERT INTO `comments` VALUES (10580,10289,11,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-09-20 10:40:01.000','2026-09-20 10:40:01.000',11,NULL,NULL);
INSERT INTO `comments` VALUES (10581,10289,11,'C\'est bon pour moi, on peut fermer.','2026-09-27 09:51:17.000','2026-09-27 09:51:17.000',11,'developpeurs',NULL);
INSERT INTO `comments` VALUES (10582,10289,2,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-10-01 16:00:00.000','2026-10-01 16:00:00.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10583,10289,6,'Corrigé dans la branche {{fix/panier-vide}}, à valider en recette.','2026-10-01 16:00:01.000','2026-10-01 16:00:01.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10584,10289,17,'Même cause que DEMO-110, je fais le lien.','2026-10-01 16:00:02.000','2026-10-01 16:00:02.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10585,10290,11,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-09-23 13:13:49.000','2026-09-23 14:08:00.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10586,10290,13,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-09-25 11:06:31.000','2026-09-25 11:06:31.000',13,NULL,NULL);
INSERT INTO `comments` VALUES (10587,10290,16,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-10-01 16:00:00.000','2026-10-01 16:00:00.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10588,10290,17,'Même cause que WEB-73, je fais le lien.','2026-10-01 16:00:01.000','2026-10-01 16:00:01.000',17,NULL,NULL);
INSERT INTO `comments` VALUES (10589,10292,15,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-09-27 08:38:48.000','2026-09-27 08:38:48.000',15,NULL,NULL);
INSERT INTO `comments` VALUES (10590,10292,10,'C\'est bon pour moi, on peut fermer.','2026-09-29 16:22:45.000','2026-09-30 12:44:25.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10591,10292,18,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-10-01 16:00:00.000','2026-10-01 16:00:00.000',18,NULL,NULL);
INSERT INTO `comments` VALUES (10592,10292,3,'Point fait avec le client ce matin, il valide la solution proposée.','2026-10-01 16:00:01.000','2026-10-01 16:00:02.000',3,NULL,NULL);
INSERT INTO `comments` VALUES (10593,10293,19,'C\'est bon pour moi, on peut fermer.','2026-09-28 16:34:19.000','2026-09-28 16:34:19.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10594,10294,19,'[~maxime.bertrand] tu peux regarder quand tu as un moment ?','2026-10-01 16:00:00.000','2026-10-01 16:00:00.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10595,10294,6,'Logs de la nuit :\n{noformat}\nWARN quota disque à 92 %\n{noformat}','2026-10-01 16:00:01.000','2026-10-01 16:00:01.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10596,10294,19,'Attention : *ne pas déployer avant vendredi*, la migration n\'est pas prête.','2026-10-01 16:00:02.000','2026-10-01 16:00:02.000',19,NULL,NULL);
INSERT INTO `comments` VALUES (10597,10294,3,'Même cause que WEB-51, je fais le lien.','2026-10-01 16:00:03.000','2026-10-01 16:00:04.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10598,10295,2,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-10-01 16:00:00.000','2026-10-01 16:00:01.000',1,NULL,NULL);
INSERT INTO `comments` VALUES (10599,10295,2,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-10-01 16:00:01.000','2026-10-01 16:00:01.000',2,NULL,NULL);
INSERT INTO `comments` VALUES (10600,10296,22,'Je confirme, reproduit sur l\'environnement de recette.','2026-10-01 16:00:00.000','2026-10-01 16:00:00.000',22,NULL,NULL);
INSERT INTO `comments` VALUES (10601,10296,22,'[~julien.roux] tu peux regarder quand tu as un moment ?','2026-10-01 16:00:01.000','2026-10-01 16:00:01.000',22,NULL,NULL);
INSERT INTO `comments` VALUES (10602,10297,6,'Je confirme, reproduit sur l\'environnement de recette.','2026-09-30 11:46:21.000','2026-09-30 11:46:21.000',6,NULL,NULL);
INSERT INTO `comments` VALUES (10603,10297,10,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-10-01 16:00:00.000','2026-10-01 16:00:00.000',10,NULL,NULL);
INSERT INTO `comments` VALUES (10604,10297,16,'[~paul.martin] tu peux regarder quand tu as un moment ?','2026-10-01 16:00:01.000','2026-10-01 16:00:01.000',16,NULL,NULL);
INSERT INTO `comments` VALUES (10605,10298,3,'Pas reproduit chez moi. Tu as une capture ou les logs ?','2026-10-01 16:00:00.000','2026-10-01 16:00:00.000',3,NULL,NULL);
INSERT INTO `comments` VALUES (10606,10299,1,'Estimation revue à la hausse : il faut aussi reprendre l\'export.','2026-10-01 16:00:00.000','2026-10-01 16:00:00.000',1,NULL,NULL);
/*!40000 ALTER TABLE `comments` ENABLE KEYS */;
DROP TABLE IF EXISTS `components`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `components` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `project_id` bigint(20) NOT NULL,
  `name` varchar(255) NOT NULL,
  `description` text DEFAULT NULL,
  `lead_id` bigint(20) DEFAULT NULL,
  `archived` tinyint(1) NOT NULL DEFAULT 0,
  `assignee_type` varchar(20) NOT NULL DEFAULT 'project_default',
  PRIMARY KEY (`id`),
  KEY `ix_components_project` (`project_id`),
  KEY `fk_components_lead` (`lead_id`),
  CONSTRAINT `fk_components_lead` FOREIGN KEY (`lead_id`) REFERENCES `users` (`id`) ON DELETE SET NULL,
  CONSTRAINT `fk_components_project` FOREIGN KEY (`project_id`) REFERENCES `projects` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=10010 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `components` DISABLE KEYS */;
INSERT INTO `components` VALUES (10000,10000,'API','Composant API',11,0,'project_default');
INSERT INTO `components` VALUES (10001,10000,'Interface','Composant Interface',7,0,'project_default');
INSERT INTO `components` VALUES (10002,10000,'Paiement','Composant Paiement',6,0,'project_default');
INSERT INTO `components` VALUES (10003,10001,'Front','Composant Front',15,0,'project_default');
INSERT INTO `components` VALUES (10004,10001,'SEO','Composant SEO',6,1,'project_default');
INSERT INTO `components` VALUES (10005,10002,'Recrutement','Composant Recrutement',8,0,'project_default');
INSERT INTO `components` VALUES (10006,10002,'Paie','Composant Paie',8,0,'project_default');
INSERT INTO `components` VALUES (10007,10003,'Serveurs','Composant Serveurs',15,0,'project_default');
INSERT INTO `components` VALUES (10008,10003,'Réseau','Composant Réseau',21,0,'project_default');
INSERT INTO `components` VALUES (10009,10003,'Sauvegardes','Composant Sauvegardes',15,0,'project_default');
/*!40000 ALTER TABLE `components` ENABLE KEYS */;
DROP TABLE IF EXISTS `custom_field_issue_types`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `custom_field_issue_types` (
  `field_id` bigint(20) NOT NULL,
  `issue_type_id` varchar(60) NOT NULL,
  PRIMARY KEY (`field_id`,`issue_type_id`),
  KEY `fk_cfit_type` (`issue_type_id`),
  CONSTRAINT `fk_cfit_field` FOREIGN KEY (`field_id`) REFERENCES `custom_fields` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_cfit_type` FOREIGN KEY (`issue_type_id`) REFERENCES `issue_types` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `custom_field_issue_types` DISABLE KEYS */;
/*!40000 ALTER TABLE `custom_field_issue_types` ENABLE KEYS */;
DROP TABLE IF EXISTS `custom_field_options`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `custom_field_options` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `field_id` bigint(20) NOT NULL,
  `parent_id` bigint(20) DEFAULT NULL,
  `position` int(11) NOT NULL DEFAULT 0,
  `value` varchar(255) NOT NULL,
  `disabled` tinyint(1) NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `ix_cfo_field` (`field_id`),
  KEY `fk_cfo_parent` (`parent_id`),
  CONSTRAINT `fk_cfo_field` FOREIGN KEY (`field_id`) REFERENCES `custom_fields` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_cfo_parent` FOREIGN KEY (`parent_id`) REFERENCES `custom_field_options` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=10225 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `custom_field_options` DISABLE KEYS */;
INSERT INTO `custom_field_options` VALUES (10200,10101,NULL,0,'Production',0);
INSERT INTO `custom_field_options` VALUES (10201,10101,NULL,1,'Recette',0);
INSERT INTO `custom_field_options` VALUES (10202,10101,NULL,2,'Développement',0);
INSERT INTO `custom_field_options` VALUES (10203,10101,NULL,3,'Préproduction',1);
INSERT INTO `custom_field_options` VALUES (10210,10102,NULL,0,'Front',0);
INSERT INTO `custom_field_options` VALUES (10211,10102,NULL,1,'Back',0);
INSERT INTO `custom_field_options` VALUES (10212,10102,NULL,2,'Mobile',0);
INSERT INTO `custom_field_options` VALUES (10213,10102,NULL,3,'Data',0);
INSERT INTO `custom_field_options` VALUES (10220,10103,NULL,0,'France',0);
INSERT INTO `custom_field_options` VALUES (10221,10103,10220,0,'Paris',0);
INSERT INTO `custom_field_options` VALUES (10222,10103,10220,1,'Lyon',0);
INSERT INTO `custom_field_options` VALUES (10223,10103,NULL,1,'Belgique',0);
INSERT INTO `custom_field_options` VALUES (10224,10103,10223,0,'Bruxelles',0);
/*!40000 ALTER TABLE `custom_field_options` ENABLE KEYS */;
DROP TABLE IF EXISTS `custom_field_projects`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `custom_field_projects` (
  `field_id` bigint(20) NOT NULL,
  `project_id` bigint(20) NOT NULL,
  PRIMARY KEY (`field_id`,`project_id`),
  KEY `ix_cfp_project` (`project_id`),
  CONSTRAINT `fk_cfp_field` FOREIGN KEY (`field_id`) REFERENCES `custom_fields` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_cfp_project` FOREIGN KEY (`project_id`) REFERENCES `projects` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `custom_field_projects` DISABLE KEYS */;
/*!40000 ALTER TABLE `custom_field_projects` ENABLE KEYS */;
DROP TABLE IF EXISTS `custom_field_values`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `custom_field_values` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `issue_id` bigint(20) NOT NULL,
  `field_id` bigint(20) NOT NULL,
  `parent_key` varchar(255) DEFAULT NULL,
  `string_value` varchar(255) DEFAULT NULL,
  `number_value` double DEFAULT NULL,
  `text_value` longtext DEFAULT NULL,
  `date_value` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_cfv_issue` (`issue_id`),
  KEY `ix_cfv_field` (`field_id`,`string_value`),
  CONSTRAINT `fk_cfv_field` FOREIGN KEY (`field_id`) REFERENCES `custom_fields` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_cfv_issue` FOREIGN KEY (`issue_id`) REFERENCES `issues` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=10508 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `custom_field_values` DISABLE KEYS */;
INSERT INTO `custom_field_values` VALUES (10000,10000,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10001,10000,10108,NULL,'CLI-7075',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10002,10001,10108,NULL,'CLI-0095',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10003,10004,10108,NULL,'CLI-5382',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10004,10005,10104,NULL,NULL,NULL,NULL,'2025-09-24 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10005,10006,10104,NULL,NULL,NULL,NULL,'2025-11-14 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10006,10007,10104,NULL,NULL,NULL,NULL,'2025-10-13 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10007,10007,10108,NULL,'CLI-9045',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10008,10008,10106,NULL,NULL,NULL,'Corrigé dans cette version : En tant que commercial, je veux voir l\'historique de mes demandes.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10009,10009,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10010,10009,10101,NULL,'10202',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10011,10012,10101,NULL,'10202',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10012,10013,10101,NULL,'10201',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10013,10015,10101,NULL,'10201',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10014,10016,10100,NULL,NULL,3,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10015,10017,10108,NULL,'CLI-0494',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10016,10018,10100,NULL,NULL,5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10017,10018,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10018,10018,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10019,10018,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10020,10018,10104,NULL,NULL,NULL,NULL,'2025-10-31 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10021,10019,10100,NULL,NULL,5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10022,10019,10101,NULL,'10201',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10023,10019,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10024,10019,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10025,10020,10103,NULL,'10220',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10026,10020,10103,'1','10222',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10027,10023,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10028,10024,10103,NULL,'10220',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10029,10024,10103,'1','10221',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10030,10025,10104,NULL,NULL,NULL,NULL,'2025-11-13 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10031,10026,10108,NULL,'CLI-3480',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10032,10027,10101,NULL,'10200',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10033,10027,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10034,10027,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10035,10027,10104,NULL,NULL,NULL,NULL,'2025-11-30 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10036,10027,10106,NULL,NULL,NULL,'Corrigé dans cette version : La page de connexion renvoie une erreur 500.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10037,10028,10105,NULL,'1',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10038,10029,10101,NULL,'10200',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10039,10029,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10040,10030,10108,NULL,'CLI-2794',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10041,10033,10101,NULL,'10202',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10042,10033,10104,NULL,NULL,NULL,NULL,'2025-12-06 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10043,10034,10108,NULL,'CLI-2229',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10044,10035,10100,NULL,NULL,5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10045,10035,10104,NULL,NULL,NULL,NULL,'2025-12-27 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10046,10035,10106,NULL,NULL,NULL,'Corrigé dans cette version : En tant que administrateur, je veux recevoir une alerte de stock.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10047,10035,10107,NULL,'https://maquettes.exemple.fr/web/8',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10048,10036,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10049,10036,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10050,10036,10104,NULL,NULL,NULL,NULL,'2025-11-26 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10051,10037,10100,NULL,NULL,0.5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10052,10038,10100,NULL,NULL,13,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10053,10038,10101,NULL,'10201',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10054,10039,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10055,10040,10106,NULL,NULL,NULL,'Corrigé dans cette version : Mettre à jour la documentation.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10056,10041,10100,NULL,NULL,13,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10057,10041,10101,NULL,'10201',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10058,10041,10108,NULL,'CLI-9248',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10059,10043,10103,NULL,'10220',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10060,10043,10103,'1','10221',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10061,10044,10106,NULL,NULL,NULL,'Corrigé dans cette version : Revue de code.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10062,10045,10100,NULL,NULL,13,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10063,10045,10106,NULL,NULL,NULL,'Corrigé dans cette version : En tant que administrateur, je veux partager un panier.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10064,10046,10100,NULL,NULL,5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10065,10046,10101,NULL,'10202',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10066,10047,10103,NULL,'10223',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10067,10047,10103,'1','10224',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10068,10048,10105,NULL,'7',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10069,10049,10100,NULL,NULL,5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10070,10050,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10071,10050,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10072,10050,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10073,10050,10106,NULL,NULL,NULL,'Corrigé dans cette version : Internationalisation.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10074,10052,10100,NULL,NULL,5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10075,10052,10107,NULL,'https://maquettes.exemple.fr/web/13',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10076,10053,10103,NULL,'10220',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10077,10053,10103,'1','10221',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10078,10055,10107,NULL,'https://maquettes.exemple.fr/web/14',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10079,10056,10101,NULL,'10202',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10080,10056,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10081,10056,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10082,10056,10104,NULL,NULL,NULL,NULL,'2025-12-17 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10083,10057,10103,NULL,'10220',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10084,10057,10103,'1','10221',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10085,10058,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10086,10059,10100,NULL,NULL,3,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10087,10059,10101,NULL,'10201',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10088,10060,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10089,10060,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10090,10060,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10091,10060,10104,NULL,NULL,NULL,NULL,'2026-01-09 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10092,10061,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10093,10062,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10094,10062,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10095,10062,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10096,10062,10108,NULL,'CLI-2317',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10097,10063,10101,NULL,'10200',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10098,10063,10104,NULL,NULL,NULL,NULL,'2026-02-02 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10099,10064,10100,NULL,NULL,5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10100,10064,10104,NULL,NULL,NULL,NULL,'2026-01-10 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10101,10066,10103,NULL,'10220',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10102,10066,10103,'1','10222',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10103,10067,10105,NULL,'23',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10104,10068,10100,NULL,NULL,5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10105,10069,10100,NULL,NULL,5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10106,10070,10103,NULL,'10223',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10107,10070,10103,'1','10224',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10108,10071,10105,NULL,'8',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10109,10072,10100,NULL,NULL,0.5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10110,10072,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10111,10072,10108,NULL,'CLI-1110',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10112,10074,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10113,10074,10104,NULL,NULL,NULL,NULL,'2026-01-11 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10114,10075,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10115,10075,10101,NULL,'10200',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10116,10076,10104,NULL,NULL,NULL,NULL,'2026-02-04 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10117,10076,10107,NULL,'https://maquettes.exemple.fr/web/17',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10118,10077,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10119,10077,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10120,10077,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10121,10077,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10122,10079,10103,NULL,'10220',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10123,10079,10103,'1','10222',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10124,10079,10105,NULL,'1',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10125,10080,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10126,10080,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10127,10081,10100,NULL,NULL,5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10128,10081,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10129,10081,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10130,10081,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10131,10083,10106,NULL,NULL,NULL,'Corrigé dans cette version : En tant que client, je veux exporter mes factures.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10132,10083,10107,NULL,'https://maquettes.exemple.fr/web/20',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10133,10084,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10134,10084,10107,NULL,'https://maquettes.exemple.fr/web/21',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10135,10085,10108,NULL,'CLI-8653',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10136,10087,10100,NULL,NULL,13,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10137,10087,10104,NULL,NULL,NULL,NULL,'2026-01-17 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10138,10088,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10139,10089,10104,NULL,NULL,NULL,NULL,'2026-03-14 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10140,10090,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10141,10090,10101,NULL,'10202',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10142,10090,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10143,10090,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10144,10091,10100,NULL,NULL,3,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10145,10092,10100,NULL,NULL,0.5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10146,10092,10108,NULL,'CLI-7892',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10147,10093,10100,NULL,NULL,5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10148,10093,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10149,10093,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10150,10093,10108,NULL,'CLI-2258',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10151,10094,10100,NULL,NULL,3,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10152,10094,10101,NULL,'10201',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10153,10094,10104,NULL,NULL,NULL,NULL,'2026-02-25 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10154,10096,10101,NULL,'10201',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10155,10096,10103,NULL,'10220',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10156,10096,10103,'1','10222',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10157,10097,10103,NULL,'10223',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10158,10097,10103,'1','10224',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10159,10097,10105,NULL,'14',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10160,10098,10105,NULL,'8',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10161,10100,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10162,10100,10106,NULL,NULL,NULL,'Corrigé dans cette version : En tant que commercial, je veux voir l\'historique de mes demandes.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10163,10101,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10164,10101,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10165,10101,10107,NULL,'https://maquettes.exemple.fr/web/24',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10166,10103,10100,NULL,NULL,1,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10167,10103,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10168,10103,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10169,10104,10107,NULL,'https://maquettes.exemple.fr/web/26',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10170,10106,10103,NULL,'10220',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10171,10106,10103,'1','10222',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10172,10107,10100,NULL,NULL,13,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10173,10109,10100,NULL,NULL,0.5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10174,10110,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10175,10110,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10176,10110,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10177,10111,10101,NULL,'10201',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10178,10112,10100,NULL,NULL,3,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10179,10112,10104,NULL,NULL,NULL,NULL,'2026-04-07 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10180,10113,10105,NULL,'22',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10181,10114,10103,NULL,'10223',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10182,10114,10103,'1','10224',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10183,10114,10105,NULL,'1',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10184,10115,10105,NULL,'1',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10185,10117,10101,NULL,'10201',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10186,10118,10105,NULL,'12',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10187,10119,10100,NULL,NULL,5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10188,10119,10101,NULL,'10202',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10189,10119,10106,NULL,NULL,NULL,'Corrigé dans cette version : Le module de paiement se vide au rafraîchissement.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10190,10120,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10191,10120,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10192,10121,10105,NULL,'19',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10193,10122,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10194,10122,10104,NULL,NULL,NULL,NULL,'2026-04-18 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10195,10124,10104,NULL,NULL,NULL,NULL,'2026-03-17 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10196,10125,10101,NULL,'10200',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10197,10125,10105,NULL,'19',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10198,10126,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10199,10126,10104,NULL,NULL,NULL,NULL,'2026-03-12 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10200,10126,10108,NULL,'CLI-0080',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10201,10127,10107,NULL,'https://maquettes.exemple.fr/web/33',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10202,10128,10100,NULL,NULL,8,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10203,10129,10100,NULL,NULL,0.5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10204,10129,10101,NULL,'10200',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10205,10129,10104,NULL,NULL,NULL,NULL,'2026-03-18 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10206,10130,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10207,10130,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10208,10130,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10209,10131,10107,NULL,'https://maquettes.exemple.fr/web/35',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10210,10133,10108,NULL,'CLI-2223',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10211,10134,10106,NULL,NULL,NULL,'Corrigé dans cette version : En tant que client, je veux exporter mes factures.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10212,10135,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10213,10135,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10214,10135,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10215,10137,10104,NULL,NULL,NULL,NULL,'2026-04-22 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10216,10137,10107,NULL,'https://maquettes.exemple.fr/web/37',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10217,10138,10100,NULL,NULL,0.5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10218,10139,10100,NULL,NULL,3,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10219,10139,10106,NULL,NULL,NULL,'Corrigé dans cette version : En tant que commercial, je veux voir l\'historique de mes demandes.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10220,10139,10108,NULL,'CLI-5545',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10221,10140,10100,NULL,NULL,0.5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10222,10140,10107,NULL,'https://maquettes.exemple.fr/web/38',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10223,10141,10100,NULL,NULL,8,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10224,10141,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10225,10141,10104,NULL,NULL,NULL,NULL,'2026-04-17 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10226,10142,10108,NULL,'CLI-2273',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10227,10143,10103,NULL,'10220',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10228,10143,10103,'1','10221',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10229,10143,10105,NULL,'15',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10230,10144,10105,NULL,'7',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10231,10146,10100,NULL,NULL,3,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10232,10146,10101,NULL,'10200',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10233,10147,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10234,10147,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10235,10148,10101,NULL,'10201',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10236,10149,10100,NULL,NULL,0.5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10237,10149,10104,NULL,NULL,NULL,NULL,'2026-05-07 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10238,10151,10105,NULL,'14',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10239,10152,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10240,10153,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10241,10153,10107,NULL,'https://maquettes.exemple.fr/web/42',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10242,10155,10100,NULL,NULL,1,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10243,10155,10104,NULL,NULL,NULL,NULL,'2026-06-04 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10244,10157,10105,NULL,'10',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10245,10158,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10246,10158,10108,NULL,'CLI-2262',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10247,10160,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10248,10161,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10249,10161,10101,NULL,'10200',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10250,10161,10104,NULL,NULL,NULL,NULL,'2026-05-05 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10251,10162,10105,NULL,'2',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10252,10163,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10253,10165,10100,NULL,NULL,5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10254,10165,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10255,10165,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10256,10165,10104,NULL,NULL,NULL,NULL,'2026-06-06 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10257,10165,10106,NULL,NULL,NULL,'Corrigé dans cette version : En tant que commercial, je veux filtrer les commandes par statut.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10258,10165,10107,NULL,'https://maquettes.exemple.fr/web/46',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10259,10167,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10260,10167,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10261,10168,10100,NULL,NULL,3,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10262,10168,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10263,10170,10100,NULL,NULL,8,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10264,10170,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10265,10172,10100,NULL,NULL,3,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10266,10172,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10267,10172,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10268,10173,10108,NULL,'CLI-7530',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10269,10174,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10270,10174,10108,NULL,'CLI-5290',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10271,10176,10104,NULL,NULL,NULL,NULL,'2026-06-19 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10272,10177,10103,NULL,'10223',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10273,10177,10103,'1','10224',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10274,10178,10101,NULL,'10201',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10275,10178,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10276,10178,10107,NULL,'https://maquettes.exemple.fr/web/51',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10277,10179,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10278,10180,10103,NULL,'10220',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10279,10180,10103,'1','10222',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10280,10182,10100,NULL,NULL,8,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10281,10182,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10282,10182,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10283,10186,10100,NULL,NULL,8,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10284,10186,10101,NULL,'10200',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10285,10186,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10286,10186,10108,NULL,'CLI-0299',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10287,10187,10105,NULL,'19',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10288,10188,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10289,10189,10104,NULL,NULL,NULL,NULL,'2026-06-28 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10290,10189,10106,NULL,NULL,NULL,'Corrigé dans cette version : Préparer la recette.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10291,10189,10108,NULL,'CLI-1165',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10292,10190,10100,NULL,NULL,1,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10293,10190,10108,NULL,'CLI-5587',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10294,10193,10100,NULL,NULL,5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10295,10193,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10296,10195,10100,NULL,NULL,5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10297,10195,10104,NULL,NULL,NULL,NULL,'2026-07-13 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10298,10196,10106,NULL,NULL,NULL,'Corrigé dans cette version : Nouveau back-office.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10299,10198,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10300,10198,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10301,10198,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10302,10198,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10303,10199,10100,NULL,NULL,5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10304,10200,10100,NULL,NULL,13,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10305,10201,10100,NULL,NULL,13,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10306,10201,10104,NULL,NULL,NULL,NULL,'2026-07-21 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10307,10202,10100,NULL,NULL,0.5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10308,10202,10101,NULL,'10200',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10309,10203,10103,NULL,'10220',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10310,10203,10103,'1','10222',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10311,10206,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10312,10207,10107,NULL,'https://maquettes.exemple.fr/web/58',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10313,10208,10100,NULL,NULL,1,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10314,10208,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10315,10208,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10316,10208,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10317,10208,10106,NULL,NULL,NULL,'Corrigé dans cette version : En tant que administrateur, je veux réinitialiser mon mot de passe.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10318,10210,10100,NULL,NULL,3,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10319,10210,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10320,10210,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10321,10210,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10322,10210,10107,NULL,'https://maquettes.exemple.fr/web/59',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10323,10211,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10324,10211,10101,NULL,'10202',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10325,10212,10100,NULL,NULL,13,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10326,10212,10106,NULL,NULL,NULL,'Corrigé dans cette version : En tant que administrateur, je veux exporter mes factures.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10327,10217,10104,NULL,NULL,NULL,NULL,'2026-07-20 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10328,10217,10108,NULL,'CLI-9546',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10329,10218,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10330,10218,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10331,10218,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10332,10219,10100,NULL,NULL,0.5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10333,10219,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10334,10219,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10335,10221,10100,NULL,NULL,1,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10336,10221,10101,NULL,'10201',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10337,10222,10100,NULL,NULL,3,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10338,10223,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10339,10223,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10340,10223,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10341,10223,10107,NULL,'https://maquettes.exemple.fr/web/66',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10342,10225,10103,NULL,'10223',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10343,10225,10103,'1','10224',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10344,10226,10105,NULL,'15',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10345,10228,10107,NULL,'https://maquettes.exemple.fr/web/67',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10346,10229,10104,NULL,NULL,NULL,NULL,'2026-08-14 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10347,10230,10100,NULL,NULL,3,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10348,10230,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10349,10230,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10350,10232,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10351,10232,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10352,10234,10103,NULL,'10223',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10353,10234,10103,'1','10224',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10354,10235,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10355,10236,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10356,10236,10101,NULL,'10200',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10357,10236,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10358,10236,10104,NULL,NULL,NULL,NULL,'2026-07-24 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10359,10236,10106,NULL,NULL,NULL,'Corrigé dans cette version : La page de connexion se vide au rafraîchissement.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10360,10236,10107,NULL,'https://maquettes.exemple.fr/web/71',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10361,10237,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10362,10237,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10363,10238,10108,NULL,'CLI-8930',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10364,10239,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10365,10239,10101,NULL,'10201',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10366,10239,10108,NULL,'CLI-2678',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10367,10240,10100,NULL,NULL,8,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10368,10241,10100,NULL,NULL,13,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10369,10241,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10370,10241,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10371,10241,10108,NULL,'CLI-7477',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10372,10242,10100,NULL,NULL,3,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10373,10242,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10374,10242,10104,NULL,NULL,NULL,NULL,'2026-08-29 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10375,10245,10100,NULL,NULL,3,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10376,10245,10108,NULL,'CLI-5438',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10377,10246,10100,NULL,NULL,3,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10378,10248,10105,NULL,'23',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10379,10249,10108,NULL,'CLI-1788',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10380,10251,10100,NULL,NULL,8,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10381,10252,10101,NULL,'10201',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10382,10253,10101,NULL,'10202',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10383,10254,10100,NULL,NULL,3,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10384,10254,10101,NULL,'10202',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10385,10254,10108,NULL,'CLI-1229',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10386,10255,10103,NULL,'10220',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10387,10255,10103,'1','10221',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10388,10257,10100,NULL,NULL,13,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10389,10257,10101,NULL,'10202',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10390,10257,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10391,10258,10101,NULL,'10201',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10392,10258,10105,NULL,'9',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10393,10259,10105,NULL,'23',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10394,10262,10101,NULL,'10201',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10395,10262,10105,NULL,'12',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10396,10264,10105,NULL,'19',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10397,10265,10104,NULL,NULL,NULL,NULL,'2026-09-14 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10398,10266,10104,NULL,NULL,NULL,NULL,'2026-10-08 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10399,10266,10108,NULL,'CLI-9514',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10400,10268,10103,NULL,'10220',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10401,10268,10103,'1','10222',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10402,10268,10105,NULL,'10',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10403,10269,10100,NULL,NULL,8,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10404,10270,10103,NULL,'10223',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10405,10270,10103,'1','10224',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10406,10272,10107,NULL,'https://maquettes.exemple.fr/web/81',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10407,10273,10100,NULL,NULL,1,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10408,10274,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10409,10274,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10410,10274,10104,NULL,NULL,NULL,NULL,'2026-11-06 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10411,10274,10107,NULL,'https://maquettes.exemple.fr/web/83',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10412,10276,10101,NULL,'10200',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10413,10276,10106,NULL,NULL,NULL,'Corrigé dans cette version : Le calendrier des livraisons ignore les caractères accentués.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10414,10277,10103,NULL,'10220',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10415,10277,10103,'1','10221',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10416,10278,10105,NULL,'14',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10417,10279,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10418,10280,10100,NULL,NULL,1,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10419,10280,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10420,10280,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10421,10280,10104,NULL,NULL,NULL,NULL,'2026-10-29 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10422,10281,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10423,10281,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10424,10281,10104,NULL,NULL,NULL,NULL,'2026-09-22 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10425,10283,10103,NULL,'10220',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10426,10283,10103,'1','10222',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10427,10284,10104,NULL,NULL,NULL,NULL,'2026-09-28 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10428,10286,10104,NULL,NULL,NULL,NULL,'2026-11-02 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10429,10287,10100,NULL,NULL,5,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10430,10287,10102,NULL,'10212',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10431,10287,10102,NULL,'10213',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10432,10287,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10433,10288,10100,NULL,NULL,1,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10434,10288,10101,NULL,'10202',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10435,10288,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10436,10288,10102,NULL,'10210',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10437,10288,10104,NULL,NULL,NULL,NULL,'2026-10-09 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10438,10289,10104,NULL,NULL,NULL,NULL,'2026-10-03 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10439,10289,10108,NULL,'CLI-2125',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10440,10290,10100,NULL,NULL,3,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10441,10290,10101,NULL,'10201',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10442,10290,10107,NULL,'https://maquettes.exemple.fr/web/87',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10443,10291,10106,NULL,NULL,NULL,'Corrigé dans cette version : En tant que administrateur, je veux recevoir une alerte de stock.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10444,10292,10100,NULL,NULL,13,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10445,10292,10101,NULL,'10202',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10446,10292,10104,NULL,NULL,NULL,NULL,'2026-11-09 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10447,10292,10107,NULL,'https://maquettes.exemple.fr/web/89',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10448,10294,10104,NULL,NULL,NULL,NULL,'2026-10-11 00:00:00.000');
INSERT INTO `custom_field_values` VALUES (10449,10295,10100,NULL,NULL,2,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10450,10295,10108,NULL,'CLI-4820',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10451,10296,10103,NULL,'10223',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10452,10296,10103,'1','10224',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10453,10297,10101,NULL,'10202',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10454,10297,10106,NULL,NULL,NULL,'Corrigé dans cette version : Le panier est très lent au-delà de 1 000 lignes.\n* Aucun impact sur les données existantes',NULL);
INSERT INTO `custom_field_values` VALUES (10455,10297,10108,NULL,'CLI-5828',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10456,10298,10102,NULL,'10211',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10457,10001,10111,NULL,'5',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10458,10010,10111,NULL,'5',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10459,10013,10111,NULL,'7',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10460,10017,10111,NULL,'5',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10461,10018,10111,NULL,'1',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10462,10019,10111,NULL,'6',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10463,10029,10111,NULL,'2',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10464,10032,10111,NULL,'6',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10465,10045,10111,NULL,'5',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10466,10049,10111,NULL,'4',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10467,10061,10111,NULL,'1',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10468,10068,10111,NULL,'4',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10469,10068,10111,NULL,'5',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10470,10072,10111,NULL,'3',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10471,10074,10111,NULL,'3',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10472,10078,10111,NULL,'7',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10473,10089,10111,NULL,'2',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10474,10090,10111,NULL,'3',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10475,10098,10111,NULL,'7',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10476,10109,10111,NULL,'5',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10477,10112,10111,NULL,'6',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10478,10129,10111,NULL,'5',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10479,10134,10111,NULL,'3',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10480,10138,10111,NULL,'5',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10481,10146,10111,NULL,'4',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10482,10146,10111,NULL,'5',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10483,10152,10111,NULL,'6',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10484,10163,10111,NULL,'2',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10485,10174,10111,NULL,'6',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10486,10180,10111,NULL,'7',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10487,10181,10111,NULL,'6',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10488,10184,10111,NULL,'6',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10489,10186,10111,NULL,'3',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10490,10190,10111,NULL,'4',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10491,10190,10111,NULL,'5',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10492,10195,10111,NULL,'5',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10493,10200,10111,NULL,'4',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10494,10212,10111,NULL,'3',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10495,10217,10111,NULL,'6',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10496,10238,10111,NULL,'1',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10497,10240,10111,NULL,'3',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10498,10241,10111,NULL,'1',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10499,10247,10111,NULL,'7',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10500,10279,10111,NULL,'5',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10501,10280,10111,NULL,'5',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10502,10284,10111,NULL,'4',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10503,10284,10111,NULL,'5',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10504,10287,10111,NULL,'5',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10505,10289,10111,NULL,'1',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10506,10295,10111,NULL,'4',NULL,NULL,NULL);
INSERT INTO `custom_field_values` VALUES (10507,10297,10111,NULL,'2',NULL,NULL,NULL);
/*!40000 ALTER TABLE `custom_field_values` ENABLE KEYS */;
DROP TABLE IF EXISTS `custom_fields`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `custom_fields` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `type_key` varchar(255) NOT NULL,
  `searcher_key` varchar(255) DEFAULT NULL,
  `name` varchar(255) NOT NULL,
  `description` text DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=10114 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `custom_fields` DISABLE KEYS */;
INSERT INTO `custom_fields` VALUES (10100,'com.atlassian.jira.plugin.system.customfieldtypes:float','com.atlassian.jira.plugin.system.customfieldtypes:exactnumber','Story Points','Champ « Story Points »');
INSERT INTO `custom_fields` VALUES (10101,'com.atlassian.jira.plugin.system.customfieldtypes:select','com.atlassian.jira.plugin.system.customfieldtypes:multiselectsearcher','Environnement client','Champ « Environnement client »');
INSERT INTO `custom_fields` VALUES (10102,'com.atlassian.jira.plugin.system.customfieldtypes:multiselect','com.atlassian.jira.plugin.system.customfieldtypes:multiselectsearcher','Équipes concernées','Champ « Équipes concernées »');
INSERT INTO `custom_fields` VALUES (10103,'com.atlassian.jira.plugin.system.customfieldtypes:cascadingselect','com.atlassian.jira.plugin.system.customfieldtypes:cascadingselectsearcher','Lieu','Champ « Lieu »');
INSERT INTO `custom_fields` VALUES (10104,'com.atlassian.jira.plugin.system.customfieldtypes:datepicker','com.atlassian.jira.plugin.system.customfieldtypes:daterange','Date de recette','Champ « Date de recette »');
INSERT INTO `custom_fields` VALUES (10105,'com.atlassian.jira.plugin.system.customfieldtypes:userpicker','com.atlassian.jira.plugin.system.customfieldtypes:userpickergroupsearcher','Référent métier','Champ « Référent métier »');
INSERT INTO `custom_fields` VALUES (10106,'com.atlassian.jira.plugin.system.customfieldtypes:textarea','com.atlassian.jira.plugin.system.customfieldtypes:textsearcher','Notes de version','Champ « Notes de version »');
INSERT INTO `custom_fields` VALUES (10107,'com.atlassian.jira.plugin.system.customfieldtypes:url','com.atlassian.jira.plugin.system.customfieldtypes:exacttextsearcher','Lien maquette','Champ « Lien maquette »');
INSERT INTO `custom_fields` VALUES (10108,'com.atlassian.jira.plugin.system.customfieldtypes:textfield','com.atlassian.jira.plugin.system.customfieldtypes:textsearcher','Code client','Champ « Code client »');
INSERT INTO `custom_fields` VALUES (10109,'com.atlassian.jira.plugin.system.customfieldtypes:labels','com.atlassian.jira.plugin.system.customfieldtypes:labelsearcher','Thèmes','Champ « Thèmes »');
INSERT INTO `custom_fields` VALUES (10110,'com.pyxis.greenhopper.jira:gh-epic-link','com.pyxis.greenhopper.jira:gh-epic-link-searcher','Epic Link','Champ « Epic Link »');
INSERT INTO `custom_fields` VALUES (10111,'com.pyxis.greenhopper.jira:gh-sprint','com.pyxis.greenhopper.jira:gh-sprint-searcher','Sprint','Champ « Sprint »');
INSERT INTO `custom_fields` VALUES (10112,'com.pyxis.greenhopper.jira:gh-lexo-rank','com.pyxis.greenhopper.jira:gh-lexo-rank-searcher','Rank','Champ « Rank »');
INSERT INTO `custom_fields` VALUES (10113,'com.pyxis.greenhopper.jira:gh-global-rank','com.pyxis.greenhopper.jira:gh-global-rank-searcher','Rank (Obsolete)','Champ « Rank (Obsolete) »');
/*!40000 ALTER TABLE `custom_fields` ENABLE KEYS */;
DROP TABLE IF EXISTS `filter_favourites`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `filter_favourites` (
  `filter_id` bigint(20) NOT NULL,
  `user_id` bigint(20) NOT NULL,
  PRIMARY KEY (`filter_id`,`user_id`),
  KEY `ix_ff_user` (`user_id`),
  CONSTRAINT `fk_ff_filter` FOREIGN KEY (`filter_id`) REFERENCES `filters` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_ff_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `filter_favourites` DISABLE KEYS */;
INSERT INTO `filter_favourites` VALUES (10000,2);
INSERT INTO `filter_favourites` VALUES (10000,3);
INSERT INTO `filter_favourites` VALUES (10000,7);
INSERT INTO `filter_favourites` VALUES (10001,3);
INSERT INTO `filter_favourites` VALUES (10001,4);
INSERT INTO `filter_favourites` VALUES (10002,7);
INSERT INTO `filter_favourites` VALUES (10003,8);
/*!40000 ALTER TABLE `filter_favourites` ENABLE KEYS */;
DROP TABLE IF EXISTS `filter_shares`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `filter_shares` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `filter_id` bigint(20) NOT NULL,
  `kind` varchar(20) NOT NULL,
  `group_id` bigint(20) DEFAULT NULL,
  `project_id` bigint(20) DEFAULT NULL,
  `role_id` bigint(20) DEFAULT NULL,
  `user_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_fs_filter` (`filter_id`),
  KEY `fk_fs_group` (`group_id`),
  KEY `fk_fs_project` (`project_id`),
  KEY `fk_fs_role` (`role_id`),
  KEY `fk_fs_user` (`user_id`),
  CONSTRAINT `fk_fs_filter` FOREIGN KEY (`filter_id`) REFERENCES `filters` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_fs_group` FOREIGN KEY (`group_id`) REFERENCES `user_groups` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_fs_project` FOREIGN KEY (`project_id`) REFERENCES `projects` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_fs_role` FOREIGN KEY (`role_id`) REFERENCES `project_roles` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_fs_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=9 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `filter_shares` DISABLE KEYS */;
INSERT INTO `filter_shares` VALUES (1,10000,'everyone',NULL,NULL,NULL,NULL);
INSERT INTO `filter_shares` VALUES (2,10001,'project',NULL,10000,NULL,NULL);
INSERT INTO `filter_shares` VALUES (3,10002,'group',3,NULL,NULL,NULL);
INSERT INTO `filter_shares` VALUES (4,10003,'project',NULL,10002,10000,NULL);
INSERT INTO `filter_shares` VALUES (5,10004,'project',NULL,10000,NULL,NULL);
INSERT INTO `filter_shares` VALUES (6,10005,'project',NULL,10001,NULL,NULL);
INSERT INTO `filter_shares` VALUES (7,10006,'project',NULL,10003,NULL,NULL);
INSERT INTO `filter_shares` VALUES (8,10007,'project',NULL,10002,10000,NULL);
/*!40000 ALTER TABLE `filter_shares` ENABLE KEYS */;
DROP TABLE IF EXISTS `filter_subscriptions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `filter_subscriptions` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `filter_id` bigint(20) NOT NULL,
  `owner_id` bigint(20) NOT NULL,
  `user_id` bigint(20) DEFAULT NULL,
  `group_id` bigint(20) DEFAULT NULL,
  `frequency` varchar(10) NOT NULL,
  `days` varchar(20) NOT NULL DEFAULT '',
  `hour` tinyint(4) NOT NULL,
  `empty_too` tinyint(1) NOT NULL DEFAULT 0,
  `last_run_at` datetime(3) DEFAULT NULL,
  `next_run_at` datetime(3) NOT NULL,
  `created_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_fsub_filter` (`filter_id`),
  KEY `ix_fsub_next` (`next_run_at`),
  KEY `fk_fsub_owner` (`owner_id`),
  KEY `fk_fsub_user` (`user_id`),
  KEY `fk_fsub_group` (`group_id`),
  CONSTRAINT `fk_fsub_filter` FOREIGN KEY (`filter_id`) REFERENCES `filters` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_fsub_group` FOREIGN KEY (`group_id`) REFERENCES `user_groups` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_fsub_owner` FOREIGN KEY (`owner_id`) REFERENCES `users` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_fsub_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `filter_subscriptions` DISABLE KEYS */;
/*!40000 ALTER TABLE `filter_subscriptions` ENABLE KEYS */;
DROP TABLE IF EXISTS `filters`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `filters` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `owner_id` bigint(20) DEFAULT NULL,
  `name` varchar(255) NOT NULL,
  `description` text DEFAULT NULL,
  `query` text NOT NULL,
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_filters_owner` (`owner_id`),
  CONSTRAINT `fk_filters_owner` FOREIGN KEY (`owner_id`) REFERENCES `users` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB AUTO_INCREMENT=10008 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `filters` DISABLE KEYS */;
INSERT INTO `filters` VALUES (10000,2,'Mes tickets ouverts','Ce qui m\'est assigné et reste à faire','assignee = currentUser() AND resolution = Unresolved ORDER BY priority DESC, updated DESC','2026-10-01 18:00:00.000','2026-10-01 18:00:00.000');
INSERT INTO `filters` VALUES (10001,3,'Bugs DEMO en cours','','project = DEMO AND issuetype = Bug AND status in (\"In Progress\", \"In Review\") ORDER BY created DESC','2026-10-01 18:00:00.000','2026-10-01 18:00:00.000');
INSERT INTO `filters` VALUES (10002,7,'Créés cette semaine','Nouveaux tickets des sept derniers jours','created >= -7d ORDER BY created DESC','2026-10-01 18:00:00.000','2026-10-01 18:00:00.000');
INSERT INTO `filters` VALUES (10003,8,'RH — à traiter','','project = RH AND status = Open ORDER BY duedate ASC','2026-10-01 18:00:00.000','2026-10-01 18:00:00.000');
INSERT INTO `filters` VALUES (10004,2,'Filtre du tableau DEMO — Scrum',NULL,'project = DEMO ORDER BY Rank ASC','2026-10-01 18:00:00.000','2026-10-01 18:00:00.000');
INSERT INTO `filters` VALUES (10005,3,'Filtre du tableau WEB — Kanban',NULL,'project = WEB ORDER BY Rank ASC','2026-10-01 18:00:00.000','2026-10-01 18:00:00.000');
INSERT INTO `filters` VALUES (10006,7,'Filtre du tableau OPS — Exploitation',NULL,'project = OPS ORDER BY Rank ASC','2026-10-01 18:00:00.000','2026-10-01 18:00:00.000');
INSERT INTO `filters` VALUES (10007,8,'Filtre du tableau RH',NULL,'project = RH ORDER BY Rank ASC','2026-10-01 18:00:00.000','2026-10-01 18:00:00.000');
/*!40000 ALTER TABLE `filters` ENABLE KEYS */;
DROP TABLE IF EXISTS `group_members`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `group_members` (
  `group_id` bigint(20) NOT NULL,
  `user_id` bigint(20) NOT NULL,
  PRIMARY KEY (`group_id`,`user_id`),
  KEY `ix_group_members_user` (`user_id`),
  CONSTRAINT `fk_gm_group` FOREIGN KEY (`group_id`) REFERENCES `user_groups` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_gm_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `group_members` DISABLE KEYS */;
INSERT INTO `group_members` VALUES (1,1);
INSERT INTO `group_members` VALUES (1,2);
INSERT INTO `group_members` VALUES (2,1);
INSERT INTO `group_members` VALUES (2,2);
INSERT INTO `group_members` VALUES (2,3);
INSERT INTO `group_members` VALUES (2,4);
INSERT INTO `group_members` VALUES (2,5);
INSERT INTO `group_members` VALUES (2,6);
INSERT INTO `group_members` VALUES (2,7);
INSERT INTO `group_members` VALUES (2,8);
INSERT INTO `group_members` VALUES (2,9);
INSERT INTO `group_members` VALUES (2,10);
INSERT INTO `group_members` VALUES (2,11);
INSERT INTO `group_members` VALUES (2,12);
INSERT INTO `group_members` VALUES (2,13);
INSERT INTO `group_members` VALUES (2,14);
INSERT INTO `group_members` VALUES (2,15);
INSERT INTO `group_members` VALUES (2,16);
INSERT INTO `group_members` VALUES (2,17);
INSERT INTO `group_members` VALUES (2,18);
INSERT INTO `group_members` VALUES (2,19);
INSERT INTO `group_members` VALUES (2,20);
INSERT INTO `group_members` VALUES (3,3);
INSERT INTO `group_members` VALUES (3,4);
INSERT INTO `group_members` VALUES (3,5);
INSERT INTO `group_members` VALUES (3,6);
INSERT INTO `group_members` VALUES (3,7);
INSERT INTO `group_members` VALUES (3,9);
INSERT INTO `group_members` VALUES (3,11);
INSERT INTO `group_members` VALUES (3,13);
INSERT INTO `group_members` VALUES (3,15);
INSERT INTO `group_members` VALUES (3,16);
INSERT INTO `group_members` VALUES (3,17);
INSERT INTO `group_members` VALUES (4,6);
INSERT INTO `group_members` VALUES (4,13);
INSERT INTO `group_members` VALUES (4,16);
INSERT INTO `group_members` VALUES (5,8);
INSERT INTO `group_members` VALUES (5,10);
INSERT INTO `group_members` VALUES (5,14);
INSERT INTO `group_members` VALUES (6,2);
INSERT INTO `group_members` VALUES (6,12);
INSERT INTO `group_members` VALUES (6,19);
INSERT INTO `group_members` VALUES (7,21);
INSERT INTO `group_members` VALUES (7,22);
INSERT INTO `group_members` VALUES (7,23);
/*!40000 ALTER TABLE `group_members` ENABLE KEYS */;
DROP TABLE IF EXISTS `issue_components`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `issue_components` (
  `issue_id` bigint(20) NOT NULL,
  `component_id` bigint(20) NOT NULL,
  PRIMARY KEY (`issue_id`,`component_id`),
  KEY `ix_ic_component` (`component_id`),
  CONSTRAINT `fk_ic_component` FOREIGN KEY (`component_id`) REFERENCES `components` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_ic_issue` FOREIGN KEY (`issue_id`) REFERENCES `issues` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `issue_components` DISABLE KEYS */;
INSERT INTO `issue_components` VALUES (10000,10002);
INSERT INTO `issue_components` VALUES (10001,10001);
INSERT INTO `issue_components` VALUES (10001,10002);
INSERT INTO `issue_components` VALUES (10002,10000);
INSERT INTO `issue_components` VALUES (10003,10002);
INSERT INTO `issue_components` VALUES (10004,10002);
INSERT INTO `issue_components` VALUES (10006,10000);
INSERT INTO `issue_components` VALUES (10007,10000);
INSERT INTO `issue_components` VALUES (10007,10002);
INSERT INTO `issue_components` VALUES (10012,10003);
INSERT INTO `issue_components` VALUES (10013,10006);
INSERT INTO `issue_components` VALUES (10014,10005);
INSERT INTO `issue_components` VALUES (10016,10000);
INSERT INTO `issue_components` VALUES (10018,10002);
INSERT INTO `issue_components` VALUES (10019,10000);
INSERT INTO `issue_components` VALUES (10019,10001);
INSERT INTO `issue_components` VALUES (10020,10008);
INSERT INTO `issue_components` VALUES (10021,10009);
INSERT INTO `issue_components` VALUES (10022,10003);
INSERT INTO `issue_components` VALUES (10027,10001);
INSERT INTO `issue_components` VALUES (10028,10005);
INSERT INTO `issue_components` VALUES (10028,10006);
INSERT INTO `issue_components` VALUES (10029,10000);
INSERT INTO `issue_components` VALUES (10030,10002);
INSERT INTO `issue_components` VALUES (10032,10000);
INSERT INTO `issue_components` VALUES (10032,10002);
INSERT INTO `issue_components` VALUES (10034,10002);
INSERT INTO `issue_components` VALUES (10035,10003);
INSERT INTO `issue_components` VALUES (10040,10002);
INSERT INTO `issue_components` VALUES (10042,10003);
INSERT INTO `issue_components` VALUES (10043,10007);
INSERT INTO `issue_components` VALUES (10044,10003);
INSERT INTO `issue_components` VALUES (10045,10002);
INSERT INTO `issue_components` VALUES (10046,10001);
INSERT INTO `issue_components` VALUES (10046,10002);
INSERT INTO `issue_components` VALUES (10047,10009);
INSERT INTO `issue_components` VALUES (10050,10001);
INSERT INTO `issue_components` VALUES (10054,10000);
INSERT INTO `issue_components` VALUES (10056,10000);
INSERT INTO `issue_components` VALUES (10056,10001);
INSERT INTO `issue_components` VALUES (10057,10007);
INSERT INTO `issue_components` VALUES (10057,10009);
INSERT INTO `issue_components` VALUES (10058,10000);
INSERT INTO `issue_components` VALUES (10059,10000);
INSERT INTO `issue_components` VALUES (10059,10002);
INSERT INTO `issue_components` VALUES (10060,10000);
INSERT INTO `issue_components` VALUES (10065,10009);
INSERT INTO `issue_components` VALUES (10070,10008);
INSERT INTO `issue_components` VALUES (10071,10006);
INSERT INTO `issue_components` VALUES (10072,10000);
INSERT INTO `issue_components` VALUES (10072,10001);
INSERT INTO `issue_components` VALUES (10073,10003);
INSERT INTO `issue_components` VALUES (10076,10003);
INSERT INTO `issue_components` VALUES (10078,10006);
INSERT INTO `issue_components` VALUES (10080,10000);
INSERT INTO `issue_components` VALUES (10080,10002);
INSERT INTO `issue_components` VALUES (10083,10003);
INSERT INTO `issue_components` VALUES (10085,10000);
INSERT INTO `issue_components` VALUES (10086,10009);
INSERT INTO `issue_components` VALUES (10090,10001);
INSERT INTO `issue_components` VALUES (10091,10001);
INSERT INTO `issue_components` VALUES (10092,10001);
INSERT INTO `issue_components` VALUES (10095,10005);
INSERT INTO `issue_components` VALUES (10095,10006);
INSERT INTO `issue_components` VALUES (10096,10008);
INSERT INTO `issue_components` VALUES (10099,10007);
INSERT INTO `issue_components` VALUES (10100,10003);
INSERT INTO `issue_components` VALUES (10105,10002);
INSERT INTO `issue_components` VALUES (10106,10005);
INSERT INTO `issue_components` VALUES (10106,10006);
INSERT INTO `issue_components` VALUES (10109,10000);
INSERT INTO `issue_components` VALUES (10109,10001);
INSERT INTO `issue_components` VALUES (10110,10002);
INSERT INTO `issue_components` VALUES (10111,10003);
INSERT INTO `issue_components` VALUES (10114,10005);
INSERT INTO `issue_components` VALUES (10116,10007);
INSERT INTO `issue_components` VALUES (10117,10003);
INSERT INTO `issue_components` VALUES (10118,10008);
INSERT INTO `issue_components` VALUES (10120,10003);
INSERT INTO `issue_components` VALUES (10124,10003);
INSERT INTO `issue_components` VALUES (10125,10008);
INSERT INTO `issue_components` VALUES (10126,10000);
INSERT INTO `issue_components` VALUES (10126,10001);
INSERT INTO `issue_components` VALUES (10127,10003);
INSERT INTO `issue_components` VALUES (10128,10003);
INSERT INTO `issue_components` VALUES (10129,10002);
INSERT INTO `issue_components` VALUES (10132,10009);
INSERT INTO `issue_components` VALUES (10133,10002);
INSERT INTO `issue_components` VALUES (10134,10002);
INSERT INTO `issue_components` VALUES (10138,10001);
INSERT INTO `issue_components` VALUES (10139,10001);
INSERT INTO `issue_components` VALUES (10140,10003);
INSERT INTO `issue_components` VALUES (10142,10000);
INSERT INTO `issue_components` VALUES (10144,10009);
INSERT INTO `issue_components` VALUES (10145,10003);
INSERT INTO `issue_components` VALUES (10147,10001);
INSERT INTO `issue_components` VALUES (10148,10007);
INSERT INTO `issue_components` VALUES (10148,10009);
INSERT INTO `issue_components` VALUES (10150,10006);
INSERT INTO `issue_components` VALUES (10151,10006);
INSERT INTO `issue_components` VALUES (10155,10001);
INSERT INTO `issue_components` VALUES (10156,10001);
INSERT INTO `issue_components` VALUES (10156,10002);
INSERT INTO `issue_components` VALUES (10157,10005);
INSERT INTO `issue_components` VALUES (10162,10007);
INSERT INTO `issue_components` VALUES (10163,10001);
INSERT INTO `issue_components` VALUES (10167,10003);
INSERT INTO `issue_components` VALUES (10168,10003);
INSERT INTO `issue_components` VALUES (10170,10003);
INSERT INTO `issue_components` VALUES (10172,10003);
INSERT INTO `issue_components` VALUES (10173,10000);
INSERT INTO `issue_components` VALUES (10184,10001);
INSERT INTO `issue_components` VALUES (10185,10003);
INSERT INTO `issue_components` VALUES (10187,10007);
INSERT INTO `issue_components` VALUES (10188,10001);
INSERT INTO `issue_components` VALUES (10192,10002);
INSERT INTO `issue_components` VALUES (10194,10005);
INSERT INTO `issue_components` VALUES (10196,10003);
INSERT INTO `issue_components` VALUES (10197,10005);
INSERT INTO `issue_components` VALUES (10198,10003);
INSERT INTO `issue_components` VALUES (10199,10000);
INSERT INTO `issue_components` VALUES (10200,10002);
INSERT INTO `issue_components` VALUES (10203,10005);
INSERT INTO `issue_components` VALUES (10205,10009);
INSERT INTO `issue_components` VALUES (10206,10000);
INSERT INTO `issue_components` VALUES (10206,10002);
INSERT INTO `issue_components` VALUES (10207,10003);
INSERT INTO `issue_components` VALUES (10215,10003);
INSERT INTO `issue_components` VALUES (10217,10000);
INSERT INTO `issue_components` VALUES (10218,10003);
INSERT INTO `issue_components` VALUES (10219,10001);
INSERT INTO `issue_components` VALUES (10220,10009);
INSERT INTO `issue_components` VALUES (10221,10003);
INSERT INTO `issue_components` VALUES (10224,10007);
INSERT INTO `issue_components` VALUES (10227,10009);
INSERT INTO `issue_components` VALUES (10230,10003);
INSERT INTO `issue_components` VALUES (10233,10007);
INSERT INTO `issue_components` VALUES (10236,10003);
INSERT INTO `issue_components` VALUES (10239,10000);
INSERT INTO `issue_components` VALUES (10241,10000);
INSERT INTO `issue_components` VALUES (10241,10001);
INSERT INTO `issue_components` VALUES (10243,10005);
INSERT INTO `issue_components` VALUES (10246,10003);
INSERT INTO `issue_components` VALUES (10247,10005);
INSERT INTO `issue_components` VALUES (10249,10001);
INSERT INTO `issue_components` VALUES (10250,10007);
INSERT INTO `issue_components` VALUES (10250,10009);
INSERT INTO `issue_components` VALUES (10253,10003);
INSERT INTO `issue_components` VALUES (10255,10005);
INSERT INTO `issue_components` VALUES (10257,10003);
INSERT INTO `issue_components` VALUES (10260,10008);
INSERT INTO `issue_components` VALUES (10260,10009);
INSERT INTO `issue_components` VALUES (10262,10009);
INSERT INTO `issue_components` VALUES (10263,10003);
INSERT INTO `issue_components` VALUES (10265,10001);
INSERT INTO `issue_components` VALUES (10266,10001);
INSERT INTO `issue_components` VALUES (10269,10003);
INSERT INTO `issue_components` VALUES (10274,10003);
INSERT INTO `issue_components` VALUES (10275,10008);
INSERT INTO `issue_components` VALUES (10276,10001);
INSERT INTO `issue_components` VALUES (10276,10002);
INSERT INTO `issue_components` VALUES (10280,10000);
INSERT INTO `issue_components` VALUES (10280,10001);
INSERT INTO `issue_components` VALUES (10281,10003);
INSERT INTO `issue_components` VALUES (10282,10000);
INSERT INTO `issue_components` VALUES (10283,10007);
INSERT INTO `issue_components` VALUES (10283,10009);
INSERT INTO `issue_components` VALUES (10284,10002);
INSERT INTO `issue_components` VALUES (10287,10001);
INSERT INTO `issue_components` VALUES (10288,10003);
INSERT INTO `issue_components` VALUES (10290,10003);
INSERT INTO `issue_components` VALUES (10291,10003);
INSERT INTO `issue_components` VALUES (10293,10009);
INSERT INTO `issue_components` VALUES (10294,10003);
INSERT INTO `issue_components` VALUES (10298,10000);
/*!40000 ALTER TABLE `issue_components` ENABLE KEYS */;
DROP TABLE IF EXISTS `issue_links`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `issue_links` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `type_id` bigint(20) NOT NULL,
  `source_id` bigint(20) NOT NULL,
  `target_id` bigint(20) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_links_source` (`source_id`),
  KEY `ix_links_target` (`target_id`),
  KEY `fk_links_type` (`type_id`),
  CONSTRAINT `fk_links_source` FOREIGN KEY (`source_id`) REFERENCES `issues` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_links_target` FOREIGN KEY (`target_id`) REFERENCES `issues` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_links_type` FOREIGN KEY (`type_id`) REFERENCES `link_types` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=10160 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `issue_links` DISABLE KEYS */;
INSERT INTO `issue_links` VALUES (10100,10003,10107,10173);
INSERT INTO `issue_links` VALUES (10101,10003,10117,10208);
INSERT INTO `issue_links` VALUES (10102,10000,10288,10011);
INSERT INTO `issue_links` VALUES (10103,10003,10218,10185);
INSERT INTO `issue_links` VALUES (10104,10000,10041,10000);
INSERT INTO `issue_links` VALUES (10105,10000,10141,10202);
INSERT INTO `issue_links` VALUES (10106,10000,10229,10092);
INSERT INTO `issue_links` VALUES (10107,10000,10232,10063);
INSERT INTO `issue_links` VALUES (10108,10003,10141,10188);
INSERT INTO `issue_links` VALUES (10109,10003,10215,10112);
INSERT INTO `issue_links` VALUES (10110,10003,10178,10138);
INSERT INTO `issue_links` VALUES (10111,10003,10251,10059);
INSERT INTO `issue_links` VALUES (10112,10000,10177,10026);
INSERT INTO `issue_links` VALUES (10113,10001,10162,10153);
INSERT INTO `issue_links` VALUES (10114,10002,10228,10054);
INSERT INTO `issue_links` VALUES (10115,10002,10279,10168);
INSERT INTO `issue_links` VALUES (10116,10000,10184,10281);
INSERT INTO `issue_links` VALUES (10117,10000,10029,10135);
INSERT INTO `issue_links` VALUES (10118,10003,10277,10108);
INSERT INTO `issue_links` VALUES (10119,10003,10084,10001);
INSERT INTO `issue_links` VALUES (10120,10000,10034,10090);
INSERT INTO `issue_links` VALUES (10121,10002,10048,10139);
INSERT INTO `issue_links` VALUES (10122,10000,10255,10151);
INSERT INTO `issue_links` VALUES (10123,10003,10262,10242);
INSERT INTO `issue_links` VALUES (10124,10003,10204,10233);
INSERT INTO `issue_links` VALUES (10125,10003,10011,10126);
INSERT INTO `issue_links` VALUES (10126,10003,10104,10199);
INSERT INTO `issue_links` VALUES (10127,10003,10227,10214);
INSERT INTO `issue_links` VALUES (10128,10003,10088,10271);
INSERT INTO `issue_links` VALUES (10129,10003,10000,10001);
INSERT INTO `issue_links` VALUES (10130,10002,10007,10050);
INSERT INTO `issue_links` VALUES (10131,10000,10187,10172);
INSERT INTO `issue_links` VALUES (10132,10000,10236,10166);
INSERT INTO `issue_links` VALUES (10133,10002,10192,10023);
INSERT INTO `issue_links` VALUES (10134,10001,10019,10167);
INSERT INTO `issue_links` VALUES (10135,10000,10040,10123);
INSERT INTO `issue_links` VALUES (10136,10000,10291,10233);
INSERT INTO `issue_links` VALUES (10137,10000,10199,10045);
INSERT INTO `issue_links` VALUES (10138,10001,10045,10276);
INSERT INTO `issue_links` VALUES (10139,10002,10237,10038);
INSERT INTO `issue_links` VALUES (10140,10003,10061,10099);
INSERT INTO `issue_links` VALUES (10141,10003,10004,10053);
INSERT INTO `issue_links` VALUES (10142,10001,10138,10249);
INSERT INTO `issue_links` VALUES (10143,10000,10052,10030);
INSERT INTO `issue_links` VALUES (10144,10001,10068,10137);
INSERT INTO `issue_links` VALUES (10145,10003,10218,10029);
INSERT INTO `issue_links` VALUES (10146,10003,10244,10245);
INSERT INTO `issue_links` VALUES (10147,10003,10298,10184);
INSERT INTO `issue_links` VALUES (10148,10003,10077,10146);
INSERT INTO `issue_links` VALUES (10149,10003,10072,10096);
INSERT INTO `issue_links` VALUES (10150,10000,10252,10139);
INSERT INTO `issue_links` VALUES (10151,10001,10062,10075);
INSERT INTO `issue_links` VALUES (10152,10000,10080,10105);
INSERT INTO `issue_links` VALUES (10153,10001,10212,10208);
INSERT INTO `issue_links` VALUES (10154,10003,10066,10273);
INSERT INTO `issue_links` VALUES (10155,10002,10213,10170);
INSERT INTO `issue_links` VALUES (10156,10003,10242,10231);
INSERT INTO `issue_links` VALUES (10157,10003,10037,10058);
INSERT INTO `issue_links` VALUES (10158,10002,10094,10087);
INSERT INTO `issue_links` VALUES (10159,10003,10263,10103);
/*!40000 ALTER TABLE `issue_links` ENABLE KEYS */;
DROP TABLE IF EXISTS `issue_types`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `issue_types` (
  `id` varchar(60) NOT NULL,
  `name` varchar(255) NOT NULL,
  `description` text DEFAULT NULL,
  `subtask` tinyint(1) NOT NULL DEFAULT 0,
  `position` int(11) NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `issue_types` DISABLE KEYS */;
INSERT INTO `issue_types` VALUES ('1','Bug','Un problème qui empêche le produit de fonctionner normalement.',0,0);
INSERT INTO `issue_types` VALUES ('10000','Epic','Un chantier qui regroupe plusieurs tickets.',0,4);
INSERT INTO `issue_types` VALUES ('10001','Story','Un besoin exprimé du point de vue de l\'utilisateur.',0,5);
INSERT INTO `issue_types` VALUES ('3','Task','Une tâche à réaliser.',0,1);
INSERT INTO `issue_types` VALUES ('4','Improvement','L\'amélioration d\'une fonctionnalité existante.',0,2);
INSERT INTO `issue_types` VALUES ('5','Sub-task','Une partie d\'un ticket plus large.',1,3);
/*!40000 ALTER TABLE `issue_types` ENABLE KEYS */;
DROP TABLE IF EXISTS `issue_versions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `issue_versions` (
  `issue_id` bigint(20) NOT NULL,
  `version_id` bigint(20) NOT NULL,
  `kind` enum('affects','fix') NOT NULL,
  PRIMARY KEY (`issue_id`,`version_id`,`kind`),
  KEY `ix_iv_version` (`version_id`),
  CONSTRAINT `fk_iv_issue` FOREIGN KEY (`issue_id`) REFERENCES `issues` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_iv_version` FOREIGN KEY (`version_id`) REFERENCES `versions` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `issue_versions` DISABLE KEYS */;
INSERT INTO `issue_versions` VALUES (10000,10003,'fix');
INSERT INTO `issue_versions` VALUES (10001,10002,'fix');
INSERT INTO `issue_versions` VALUES (10002,10002,'fix');
INSERT INTO `issue_versions` VALUES (10003,10003,'fix');
INSERT INTO `issue_versions` VALUES (10007,10001,'fix');
INSERT INTO `issue_versions` VALUES (10009,10005,'affects');
INSERT INTO `issue_versions` VALUES (10010,10002,'fix');
INSERT INTO `issue_versions` VALUES (10018,10003,'fix');
INSERT INTO `issue_versions` VALUES (10019,10001,'affects');
INSERT INTO `issue_versions` VALUES (10025,10002,'fix');
INSERT INTO `issue_versions` VALUES (10026,10001,'fix');
INSERT INTO `issue_versions` VALUES (10027,10000,'affects');
INSERT INTO `issue_versions` VALUES (10029,10000,'affects');
INSERT INTO `issue_versions` VALUES (10037,10005,'fix');
INSERT INTO `issue_versions` VALUES (10040,10001,'fix');
INSERT INTO `issue_versions` VALUES (10041,10002,'fix');
INSERT INTO `issue_versions` VALUES (10045,10001,'fix');
INSERT INTO `issue_versions` VALUES (10049,10003,'fix');
INSERT INTO `issue_versions` VALUES (10050,10002,'fix');
INSERT INTO `issue_versions` VALUES (10052,10005,'fix');
INSERT INTO `issue_versions` VALUES (10056,10003,'fix');
INSERT INTO `issue_versions` VALUES (10059,10001,'affects');
INSERT INTO `issue_versions` VALUES (10059,10002,'fix');
INSERT INTO `issue_versions` VALUES (10060,10001,'fix');
INSERT INTO `issue_versions` VALUES (10061,10003,'fix');
INSERT INTO `issue_versions` VALUES (10064,10005,'fix');
INSERT INTO `issue_versions` VALUES (10077,10006,'fix');
INSERT INTO `issue_versions` VALUES (10080,10002,'fix');
INSERT INTO `issue_versions` VALUES (10081,10004,'fix');
INSERT INTO `issue_versions` VALUES (10083,10004,'fix');
INSERT INTO `issue_versions` VALUES (10084,10004,'fix');
INSERT INTO `issue_versions` VALUES (10085,10001,'fix');
INSERT INTO `issue_versions` VALUES (10087,10003,'fix');
INSERT INTO `issue_versions` VALUES (10088,10005,'fix');
INSERT INTO `issue_versions` VALUES (10089,10001,'fix');
INSERT INTO `issue_versions` VALUES (10091,10002,'fix');
INSERT INTO `issue_versions` VALUES (10093,10001,'fix');
INSERT INTO `issue_versions` VALUES (10094,10001,'affects');
INSERT INTO `issue_versions` VALUES (10100,10006,'fix');
INSERT INTO `issue_versions` VALUES (10101,10004,'fix');
INSERT INTO `issue_versions` VALUES (10107,10005,'fix');
INSERT INTO `issue_versions` VALUES (10108,10001,'fix');
INSERT INTO `issue_versions` VALUES (10110,10003,'fix');
INSERT INTO `issue_versions` VALUES (10111,10004,'fix');
INSERT INTO `issue_versions` VALUES (10111,10005,'affects');
INSERT INTO `issue_versions` VALUES (10117,10004,'affects');
INSERT INTO `issue_versions` VALUES (10119,10001,'affects');
INSERT INTO `issue_versions` VALUES (10126,10002,'fix');
INSERT INTO `issue_versions` VALUES (10127,10004,'affects');
INSERT INTO `issue_versions` VALUES (10127,10005,'fix');
INSERT INTO `issue_versions` VALUES (10128,10005,'fix');
INSERT INTO `issue_versions` VALUES (10129,10000,'affects');
INSERT INTO `issue_versions` VALUES (10131,10004,'affects');
INSERT INTO `issue_versions` VALUES (10134,10003,'fix');
INSERT INTO `issue_versions` VALUES (10135,10003,'fix');
INSERT INTO `issue_versions` VALUES (10138,10003,'fix');
INSERT INTO `issue_versions` VALUES (10140,10004,'fix');
INSERT INTO `issue_versions` VALUES (10141,10004,'fix');
INSERT INTO `issue_versions` VALUES (10142,10003,'fix');
INSERT INTO `issue_versions` VALUES (10146,10001,'affects');
INSERT INTO `issue_versions` VALUES (10147,10002,'fix');
INSERT INTO `issue_versions` VALUES (10153,10005,'fix');
INSERT INTO `issue_versions` VALUES (10154,10003,'fix');
INSERT INTO `issue_versions` VALUES (10155,10001,'fix');
INSERT INTO `issue_versions` VALUES (10158,10001,'fix');
INSERT INTO `issue_versions` VALUES (10160,10004,'fix');
INSERT INTO `issue_versions` VALUES (10164,10006,'fix');
INSERT INTO `issue_versions` VALUES (10167,10005,'fix');
INSERT INTO `issue_versions` VALUES (10169,10002,'fix');
INSERT INTO `issue_versions` VALUES (10170,10005,'fix');
INSERT INTO `issue_versions` VALUES (10174,10001,'fix');
INSERT INTO `issue_versions` VALUES (10178,10005,'affects');
INSERT INTO `issue_versions` VALUES (10179,10003,'fix');
INSERT INTO `issue_versions` VALUES (10183,10005,'fix');
INSERT INTO `issue_versions` VALUES (10184,10000,'affects');
INSERT INTO `issue_versions` VALUES (10184,10002,'fix');
INSERT INTO `issue_versions` VALUES (10185,10006,'fix');
INSERT INTO `issue_versions` VALUES (10186,10003,'fix');
INSERT INTO `issue_versions` VALUES (10188,10002,'fix');
INSERT INTO `issue_versions` VALUES (10189,10001,'fix');
INSERT INTO `issue_versions` VALUES (10193,10003,'fix');
INSERT INTO `issue_versions` VALUES (10196,10005,'fix');
INSERT INTO `issue_versions` VALUES (10199,10003,'fix');
INSERT INTO `issue_versions` VALUES (10202,10005,'affects');
INSERT INTO `issue_versions` VALUES (10206,10002,'fix');
INSERT INTO `issue_versions` VALUES (10210,10006,'fix');
INSERT INTO `issue_versions` VALUES (10211,10002,'fix');
INSERT INTO `issue_versions` VALUES (10212,10003,'fix');
INSERT INTO `issue_versions` VALUES (10214,10004,'fix');
INSERT INTO `issue_versions` VALUES (10216,10005,'fix');
INSERT INTO `issue_versions` VALUES (10221,10005,'affects');
INSERT INTO `issue_versions` VALUES (10221,10006,'fix');
INSERT INTO `issue_versions` VALUES (10222,10005,'fix');
INSERT INTO `issue_versions` VALUES (10223,10005,'fix');
INSERT INTO `issue_versions` VALUES (10231,10006,'fix');
INSERT INTO `issue_versions` VALUES (10235,10001,'fix');
INSERT INTO `issue_versions` VALUES (10236,10004,'affects');
INSERT INTO `issue_versions` VALUES (10236,10005,'fix');
INSERT INTO `issue_versions` VALUES (10237,10006,'fix');
INSERT INTO `issue_versions` VALUES (10245,10003,'fix');
INSERT INTO `issue_versions` VALUES (10249,10001,'fix');
INSERT INTO `issue_versions` VALUES (10253,10005,'fix');
INSERT INTO `issue_versions` VALUES (10254,10001,'affects');
INSERT INTO `issue_versions` VALUES (10257,10004,'fix');
INSERT INTO `issue_versions` VALUES (10263,10005,'fix');
INSERT INTO `issue_versions` VALUES (10271,10000,'affects');
INSERT INTO `issue_versions` VALUES (10271,10001,'fix');
INSERT INTO `issue_versions` VALUES (10272,10004,'fix');
INSERT INTO `issue_versions` VALUES (10273,10005,'fix');
INSERT INTO `issue_versions` VALUES (10276,10000,'affects');
INSERT INTO `issue_versions` VALUES (10276,10003,'fix');
INSERT INTO `issue_versions` VALUES (10280,10002,'fix');
INSERT INTO `issue_versions` VALUES (10286,10006,'fix');
INSERT INTO `issue_versions` VALUES (10287,10001,'fix');
INSERT INTO `issue_versions` VALUES (10288,10004,'affects');
INSERT INTO `issue_versions` VALUES (10289,10001,'fix');
INSERT INTO `issue_versions` VALUES (10290,10004,'affects');
INSERT INTO `issue_versions` VALUES (10290,10004,'fix');
INSERT INTO `issue_versions` VALUES (10291,10004,'fix');
INSERT INTO `issue_versions` VALUES (10292,10005,'affects');
INSERT INTO `issue_versions` VALUES (10297,10003,'fix');
INSERT INTO `issue_versions` VALUES (10298,10002,'fix');
/*!40000 ALTER TABLE `issue_versions` ENABLE KEYS */;
DROP TABLE IF EXISTS `issues`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `issues` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `project_id` bigint(20) NOT NULL,
  `num` bigint(20) NOT NULL,
  `issue_type_id` varchar(60) NOT NULL,
  `parent_id` bigint(20) DEFAULT NULL,
  `summary` varchar(255) NOT NULL,
  `description` longtext DEFAULT NULL,
  `environment` longtext DEFAULT NULL,
  `status_id` varchar(60) NOT NULL,
  `priority_id` varchar(60) DEFAULT NULL,
  `resolution_id` varchar(60) DEFAULT NULL,
  `reporter_id` bigint(20) DEFAULT NULL,
  `assignee_id` bigint(20) DEFAULT NULL,
  `creator_id` bigint(20) DEFAULT NULL,
  `security_level_id` bigint(20) DEFAULT NULL,
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  `resolved_at` datetime(3) DEFAULT NULL,
  `due_date` date DEFAULT NULL,
  `original_estimate` bigint(20) DEFAULT NULL,
  `remaining_estimate` bigint(20) DEFAULT NULL,
  `time_spent` bigint(20) DEFAULT NULL,
  `archived` tinyint(1) NOT NULL DEFAULT 0,
  `rank_key` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin DEFAULT NULL,
  `epic_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_issues_num` (`project_id`,`num`),
  KEY `ix_issues_assignee` (`assignee_id`),
  KEY `ix_issues_reporter` (`reporter_id`),
  KEY `ix_issues_status` (`status_id`),
  KEY `ix_issues_updated` (`updated_at`),
  KEY `ix_issues_parent` (`parent_id`),
  KEY `fk_issues_type` (`issue_type_id`),
  KEY `fk_issues_priority` (`priority_id`),
  KEY `fk_issues_resolution` (`resolution_id`),
  KEY `fk_issues_creator` (`creator_id`),
  KEY `fk_issues_security` (`security_level_id`),
  KEY `ix_issues_rank` (`rank_key`),
  KEY `ix_issues_epic` (`epic_id`),
  FULLTEXT KEY `ft_issues` (`summary`,`description`),
  CONSTRAINT `fk_issues_assignee` FOREIGN KEY (`assignee_id`) REFERENCES `users` (`id`) ON DELETE SET NULL,
  CONSTRAINT `fk_issues_creator` FOREIGN KEY (`creator_id`) REFERENCES `users` (`id`) ON DELETE SET NULL,
  CONSTRAINT `fk_issues_epic` FOREIGN KEY (`epic_id`) REFERENCES `issues` (`id`) ON DELETE SET NULL,
  CONSTRAINT `fk_issues_parent` FOREIGN KEY (`parent_id`) REFERENCES `issues` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_issues_priority` FOREIGN KEY (`priority_id`) REFERENCES `priorities` (`id`),
  CONSTRAINT `fk_issues_project` FOREIGN KEY (`project_id`) REFERENCES `projects` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_issues_reporter` FOREIGN KEY (`reporter_id`) REFERENCES `users` (`id`) ON DELETE SET NULL,
  CONSTRAINT `fk_issues_resolution` FOREIGN KEY (`resolution_id`) REFERENCES `resolutions` (`id`),
  CONSTRAINT `fk_issues_security` FOREIGN KEY (`security_level_id`) REFERENCES `security_levels` (`id`),
  CONSTRAINT `fk_issues_status` FOREIGN KEY (`status_id`) REFERENCES `statuses` (`id`),
  CONSTRAINT `fk_issues_type` FOREIGN KEY (`issue_type_id`) REFERENCES `issue_types` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=10300 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `issues` DISABLE KEYS */;
INSERT INTO `issues` VALUES (10000,10000,1,'3',NULL,'Mettre à jour les tests de bout en bout','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Le résultat est identique sur mobile et sur ordinateur\n* Un message clair s\'affiche en cas d\'erreur\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}',NULL,'10000','3',NULL,16,6,16,NULL,'2025-08-29 07:22:43.000','2025-09-25 11:58:36.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00k5c:',NULL);
INSERT INTO `issues` VALUES (10001,10000,2,'3',NULL,'Mettre à jour la page de profil','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}\n\n[~marie.durand] peux-tu valider ?',NULL,'10001','3','10000',10,5,10,NULL,'2025-09-01 07:42:55.000','2025-09-30 14:06:58.000','2025-09-12 06:08:42.000',NULL,21600,21600,NULL,0,'0|i02dq8:',NULL);
INSERT INTO `issues` VALUES (10002,10000,3,'3',NULL,'Refondre les tests de bout en bout','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur\n* Un message clair s\'affiche en cas d\'erreur\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}',NULL,'3','3',NULL,13,2,13,NULL,'2025-09-01 13:58:46.000','2025-09-20 14:23:40.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00kjk:',NULL);
INSERT INTO `issues` VALUES (10003,10000,4,'10000',NULL,'Refonte du tunnel de commande','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n\n[~thomas.moreau] peux-tu valider ?',NULL,'10000','3',NULL,24,NULL,24,NULL,'2025-09-10 08:50:15.000','2025-09-11 17:59:18.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00kxs:',NULL);
INSERT INTO `issues` VALUES (10004,10000,5,'3',NULL,'Simplifier l\'export CSV des factures','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* La page répond en moins d\'une seconde\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}\n\nVoir [la maquette|https://maquettes.exemple.fr/panier]. Lié à DEMO-4.',NULL,'10000','3',NULL,17,11,17,NULL,'2025-09-10 08:52:04.000','2025-12-14 18:40:36.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00lc0:',NULL);
INSERT INTO `issues` VALUES (10005,10000,6,'3',NULL,'Tester le tableau des commandes','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Les tests de bout en bout couvrent le cas nominal\n* Les droits existants sont respectés\n\nLié à DEMO-1.',NULL,'10001','3','10000',10,2,10,NULL,'2025-09-13 06:31:11.000','2025-10-03 09:47:10.000','2025-10-03 09:47:10.000',NULL,NULL,NULL,NULL,0,'0|i02e4g:',NULL);
INSERT INTO `issues` VALUES (10006,10000,7,'3',NULL,'Simplifier les tests de bout en bout','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Un message clair s\'affiche en cas d\'erreur\n* Les droits existants sont respectés\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}\n\nVoir [la documentation interne|https://wiki.exemple.fr/exploitation/sauvegardes].',NULL,'3','3',NULL,9,NULL,9,NULL,'2025-09-13 12:40:39.000','2025-09-13 15:26:42.000',NULL,'2025-10-25',NULL,NULL,NULL,0,'0|i00lq8:',NULL);
INSERT INTO `issues` VALUES (10007,10000,8,'10001',NULL,'En tant que client, je veux filtrer les commandes par statut','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Les tests de bout en bout couvrent le cas nominal\n* La page répond en moins d\'une seconde\n\n{code:sql}\nSELECT id, statut FROM commande\nWHERE cree_le > now() - interval \'7 days\';\n{code}\n\nVoir [la maquette|https://maquettes.exemple.fr/panier].',NULL,'10000','4',NULL,7,9,7,NULL,'2025-09-16 08:46:51.000','2025-12-15 13:54:17.000',NULL,'2025-10-05',NULL,NULL,NULL,0,'0|i01g5c:',NULL);
INSERT INTO `issues` VALUES (10008,10001,1,'10001',NULL,'En tant que commercial, je veux voir l\'historique de mes demandes','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* La page répond en moins d\'une seconde\n* Un message clair s\'affiche en cas d\'erreur\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}\n\n[~hugo.robert] peux-tu valider ?',NULL,'10001','2','10000',17,5,17,NULL,'2025-09-17 16:57:52.000','2025-10-12 17:29:10.000','2025-10-09 13:21:38.000',NULL,NULL,NULL,65700,0,'0|i021hc:',NULL);
INSERT INTO `issues` VALUES (10009,10001,2,'1',NULL,'L\'import CSV renvoie une erreur 500','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\n*Étapes pour reproduire :*\n# Se connecter avec un compte client\n# Lancer l\'export\n# Ajouter deux articles au panier\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\nVoir [la maquette|https://maquettes.exemple.fr/panier].','Safari 18 / iOS 18','3','4',NULL,7,NULL,7,NULL,'2025-09-20 06:59:02.000','2025-10-02 17:17:29.000',NULL,NULL,144000,144000,NULL,0,'0|i01gjk:',NULL);
INSERT INTO `issues` VALUES (10010,10000,9,'3',NULL,'Mettre à jour le système de notifications','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Les droits existants sont respectés',NULL,'10001','3','10000',24,15,24,NULL,'2025-09-21 08:37:46.000','2025-10-13 07:34:04.000','2025-10-13 07:34:04.000',NULL,NULL,NULL,NULL,0,'0|i02eio:',NULL);
INSERT INTO `issues` VALUES (10011,10001,3,'3',NULL,'Automatiser la configuration du cache','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n\n{code:sql}\nSELECT id, statut FROM commande\nWHERE cree_le > now() - interval \'7 days\';\n{code}',NULL,'10001','1','3',4,3,4,NULL,'2025-09-21 09:14:57.000','2026-03-19 14:43:31.000','2025-10-04 11:26:57.000',NULL,NULL,NULL,NULL,0,'0|i01wqo:',NULL);
INSERT INTO `issues` VALUES (10012,10001,4,'1',NULL,'L\'import CSV envoie deux fois le même message','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\n*Étapes pour reproduire :*\n# Ajouter deux articles au panier\n# Ouvrir le tableau de bord\n# Lancer l\'export\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}\n\n!capture.png!',NULL,'10000','2',NULL,18,13,18,NULL,'2025-09-23 11:23:51.000','2025-10-14 09:34:48.000',NULL,NULL,14400,14400,NULL,0,'0|i005xc:',NULL);
INSERT INTO `issues` VALUES (10013,10002,1,'1',NULL,'Planifier la formation sécurité','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\n*Étapes pour reproduire :*\n# Ajouter deux articles au panier\n# Se connecter avec un compte client\n# Ouvrir le tableau de bord\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}\n\n[~paul.martin] peux-tu valider ?','Chrome 129 / Windows 11','3','3',NULL,1,10,1,NULL,'2025-09-26 06:25:15.000','2025-10-10 17:32:26.000',NULL,NULL,NULL,NULL,65700,0,'0|i00m4g:',NULL);
INSERT INTO `issues` VALUES (10014,10002,2,'5',10013,'Rédiger les tests unitaires','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Le résultat est identique sur mobile et sur ordinateur\n\n[~lucas.bernard] peux-tu valider ?',NULL,'5','3','1',14,14,1,NULL,'2025-09-26 14:46:49.000','2025-11-15 17:02:56.000','2025-10-13 12:45:34.000',NULL,NULL,NULL,NULL,0,'0|i02eww:',NULL);
INSERT INTO `issues` VALUES (10015,10003,1,'1',NULL,'Redémarrage inopiné de srv-batch-01','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\n*Étapes pour reproduire :*\n# Ajouter deux articles au panier\n# Rafraîchir la page\n# Lancer l\'export\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n{code:sql}\nSELECT id, statut FROM commande\nWHERE cree_le > now() - interval \'7 days\';\n{code}',NULL,'5','3','1',7,11,7,NULL,'2025-09-28 12:57:01.000','2025-10-19 15:00:29.000','2025-10-08 06:16:29.000','2025-11-03',NULL,NULL,32400,0,'0|i02fb4:',NULL);
INSERT INTO `issues` VALUES (10016,10000,10,'10001',NULL,'En tant que client, je veux voir l\'historique de mes demandes','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}',NULL,'10000','3',NULL,19,NULL,19,NULL,'2025-09-30 07:19:42.000','2025-10-04 09:24:00.000',NULL,NULL,7200,7200,NULL,0,'0|i00mio:',10003);
INSERT INTO `issues` VALUES (10017,10000,11,'3',NULL,'Mettre à jour la page de profil','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Un message clair s\'affiche en cas d\'erreur\n* La page répond en moins d\'une seconde',NULL,'10000','3',NULL,24,15,24,NULL,'2025-10-02 08:29:15.000','2025-11-03 18:31:04.000',NULL,NULL,64800,64800,NULL,0,'0|i00mww:',10003);
INSERT INTO `issues` VALUES (10018,10000,12,'10001',NULL,'En tant que commercial, je veux voir l\'historique de mes demandes',NULL,NULL,'10001','3','3',6,3,1,NULL,'2025-10-05 12:36:44.000','2025-10-26 10:10:40.000','2025-10-26 10:10:40.000',NULL,NULL,NULL,NULL,0,'0|i02fpc:',10003);
INSERT INTO `issues` VALUES (10019,10000,13,'1',NULL,'La recherche avancée ignore les caractères accentués','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\n*Étapes pour reproduire :*\n# Rafraîchir la page\n# Se connecter avec un compte client\n# Lancer l\'export\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n!maquette.jpg!','Production — srv-web-01','10000','4',NULL,17,17,1,NULL,'2025-10-05 13:28:57.000','2026-04-29 15:05:41.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01gxs:',NULL);
INSERT INTO `issues` VALUES (10020,10003,2,'3',NULL,'Redémarrage inopiné de srv-web-01','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Un message clair s\'affiche en cas d\'erreur\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}',NULL,'4','2',NULL,19,7,19,NULL,'2025-10-15 10:02:20.000','2025-11-17 17:04:22.000',NULL,NULL,NULL,NULL,NULL,0,'0|i006bk:',NULL);
INSERT INTO `issues` VALUES (10021,10003,3,'4',NULL,'Mettre à jour PostgreSQL sur srv-db-02','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur\n* La page répond en moins d\'une seconde',NULL,'5','2','1',12,NULL,12,NULL,'2025-10-16 14:35:29.000','2025-10-27 14:48:16.000','2025-10-27 14:48:16.000','2025-10-25',NULL,NULL,NULL,0,'0|i021vk:',NULL);
INSERT INTO `issues` VALUES (10022,10001,5,'10001',NULL,'En tant que administrateur, je veux exporter mes factures','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Les droits existants sont respectés\n* Les tests de bout en bout couvrent le cas nominal\n\n{code:sql}\nSELECT id, statut FROM commande\nWHERE cree_le > now() - interval \'7 days\';\n{code}\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}',NULL,'10001','3','3',9,17,9,NULL,'2025-10-16 15:50:45.000','2025-11-06 17:04:32.000','2025-11-06 17:04:32.000',NULL,NULL,NULL,NULL,0,'0|i02g3k:',NULL);
INSERT INTO `issues` VALUES (10023,10001,6,'10001',NULL,'En tant que agent du support, je veux réinitialiser mon mot de passe','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Les droits existants sont respectés\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}',NULL,'10002','4',NULL,10,9,10,NULL,'2025-10-17 16:54:09.000','2025-11-13 16:05:33.000',NULL,NULL,126000,126000,NULL,0,'0|i01hc0:',NULL);
INSERT INTO `issues` VALUES (10024,10003,4,'3',NULL,'Ouvrir le flux réseau entre srv-db-02 et le VPN','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les droits existants sont respectés',NULL,'4','3',NULL,2,22,2,NULL,'2025-10-18 07:53:34.000','2025-11-13 08:36:56.000',NULL,'2025-12-03',50400,50400,NULL,0,'0|i00nb4:',NULL);
INSERT INTO `issues` VALUES (10025,10000,121,'3',NULL,'Tester l\'export CSV des factures','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal',NULL,'10000','3',NULL,5,17,5,NULL,'2025-10-18 08:27:30.000','2025-11-24 10:55:43.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00npc:',10003);
INSERT INTO `issues` VALUES (10026,10000,14,'10001',NULL,'En tant que commercial, je veux réinitialiser mon mot de passe','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Les droits existants sont respectés\n\nLié à DEMO-12. [~marie.durand] peux-tu valider ?',NULL,'10000','3',NULL,13,15,1,2,'2025-10-18 16:03:21.000','2025-11-07 15:04:27.000',NULL,NULL,43200,43200,NULL,0,'0|i00o3k:',NULL);
INSERT INTO `issues` VALUES (10027,10000,15,'1',NULL,'La page de connexion renvoie une erreur 500',NULL,'Firefox 131 / macOS 15','10001','2','10000',18,9,18,NULL,'2025-10-19 07:48:54.000','2025-11-19 11:04:09.000','2025-11-19 11:04:09.000',NULL,126000,53100,72900,0,'0|i0229s:',NULL);
INSERT INTO `issues` VALUES (10028,10002,3,'3',NULL,'Préparer l\'entretien annuel de Mme Lambert','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les droits existants sont respectés\n\n{code:sql}\nSELECT id, statut FROM commande\nWHERE cree_le > now() - interval \'7 days\';\n{code}',NULL,'3','3',NULL,10,NULL,10,NULL,'2025-10-20 11:16:10.000','2025-10-30 10:36:41.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00ohs:',NULL);
INSERT INTO `issues` VALUES (10029,10000,16,'1',NULL,'La synchronisation mobile envoie deux fois le même message','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\n*Étapes pour reproduire :*\n# Rafraîchir la page\n# Ouvrir le tableau de bord\n# Lancer l\'export\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}\n\nVoir [la documentation interne|https://wiki.exemple.fr/exploitation/sauvegardes]. Lié à DEMO-12.',NULL,'10001','3','10000',6,11,6,NULL,'2025-10-21 07:39:11.000','2026-03-20 11:22:20.000','2025-10-30 12:33:21.000',NULL,NULL,NULL,28800,0,'0|i02ghs:',10003);
INSERT INTO `issues` VALUES (10030,10000,17,'3',NULL,'Tester le tableau des commandes','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les droits existants sont respectés\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}\n\nLié à DEMO-3.',NULL,'10000','1',NULL,2,11,2,NULL,'2025-10-21 10:01:41.000','2025-10-30 15:58:09.000',NULL,'2025-12-19',75600,75600,NULL,0,'0|i00000:',NULL);
INSERT INTO `issues` VALUES (10031,10001,7,'10001',NULL,'En tant que client, je veux suivre ma livraison','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* La page répond en moins d\'une seconde\n\nLié à WEB-3. [~camille.petit] peux-tu valider ?',NULL,'10000','3',NULL,17,5,17,NULL,'2025-10-23 15:09:52.000','2025-10-31 16:45:16.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00ow0:',NULL);
INSERT INTO `issues` VALUES (10032,10000,18,'3',NULL,'Tester le tableau des commandes','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}\n\n[~camille.petit] peux-tu valider ?',NULL,'10000','1',NULL,6,16,6,NULL,'2025-10-24 10:53:55.000','2025-11-13 17:19:20.000',NULL,NULL,NULL,NULL,NULL,0,'0|i000e8:',10003);
INSERT INTO `issues` VALUES (10033,10000,19,'1',NULL,'Le panier perd les données saisies','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\n*Étapes pour reproduire :*\n# Lancer l\'export\n# Ouvrir le tableau de bord\n# Se connecter avec un compte client\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n[~marie.durand] peux-tu valider ?','Chrome 129 / Windows 11','10000','4',NULL,3,11,1,NULL,'2025-10-25 11:18:37.000','2025-11-14 14:57:53.000',NULL,'2025-11-19',NULL,NULL,NULL,0,'0|i01hq8:',NULL);
INSERT INTO `issues` VALUES (10034,10000,20,'5',10010,'Mettre à jour la documentation','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Les tests de bout en bout couvrent le cas nominal\n\nLié à DEMO-9. [~paul.martin] peux-tu valider ?',NULL,'10000','5',NULL,13,16,13,NULL,'2025-10-30 12:09:20.000','2026-01-29 09:25:41.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01tz4:',NULL);
INSERT INTO `issues` VALUES (10035,10001,8,'10001',NULL,'En tant que administrateur, je veux recevoir une alerte de stock','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Les tests de bout en bout couvrent le cas nominal\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412]. [~julien.roux] peux-tu valider ?\n\n!capture.png!',NULL,'10001','2','10000',3,5,3,NULL,'2025-11-01 08:40:48.000','2025-11-28 10:07:24.000','2025-11-28 10:07:24.000',NULL,NULL,NULL,NULL,0,'0|i022o0:',NULL);
INSERT INTO `issues` VALUES (10036,10000,21,'10001',NULL,'En tant que client, je veux filtrer les commandes par statut','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Les droits existants sont respectés\n* La page répond en moins d\'une seconde\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}',NULL,'10002','4',NULL,18,15,18,NULL,'2025-11-01 12:17:13.000','2025-11-21 17:09:14.000',NULL,NULL,140400,140400,NULL,0,'0|i01i4g:',NULL);
INSERT INTO `issues` VALUES (10037,10001,9,'10001',NULL,'En tant que agent du support, je veux recevoir une alerte de stock','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* La page répond en moins d\'une seconde\n\n[~ines.david] peux-tu valider ?',NULL,'10000','3',NULL,9,9,1,NULL,'2025-11-03 09:30:37.000','2025-12-10 08:34:45.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00pa8:',NULL);
INSERT INTO `issues` VALUES (10038,10000,22,'1',NULL,'Le module de paiement plante sur Safari 18','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\n*Étapes pour reproduire :*\n# Rafraîchir la page\n# Changer la langue en anglais\n# Ajouter deux articles au panier\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.',NULL,'3','4',NULL,12,3,12,NULL,'2025-11-06 09:04:27.000','2025-11-21 14:39:39.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01iio:',10003);
INSERT INTO `issues` VALUES (10039,10001,10,'3',NULL,'Simplifier la gestion des rôles','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Le résultat est identique sur mobile et sur ordinateur\n* Un message clair s\'affiche en cas d\'erreur\n\nLié à WEB-8.',NULL,'3','1',NULL,4,4,4,NULL,'2025-11-06 18:49:07.000','2025-12-02 18:19:05.000',NULL,'2025-12-05',NULL,NULL,51300,0,'0|i000sg:',NULL);
INSERT INTO `issues` VALUES (10040,10000,23,'5',10018,'Mettre à jour la documentation','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde',NULL,'10001','1','10000',9,NULL,9,NULL,'2025-11-07 08:05:53.000','2026-02-27 07:23:56.000','2025-11-18 18:04:16.000',NULL,NULL,NULL,54000,0,'0|i01x4w:',NULL);
INSERT INTO `issues` VALUES (10041,10000,24,'1',NULL,'Les notifications e-mail envoie deux fois le même message','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\n*Étapes pour reproduire :*\n# Rafraîchir la page\n# Ouvrir le tableau de bord\n# Lancer l\'export\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\nLié à DEMO-19. [~nicolas.laurent] peux-tu valider ?','Production — srv-web-01','10001','3','3',18,4,18,NULL,'2025-11-08 08:05:25.000','2025-12-02 11:16:30.000','2025-12-01 10:49:01.000',NULL,133200,133200,NULL,0,'0|i02gw0:',NULL);
INSERT INTO `issues` VALUES (10042,10001,11,'5',10009,'Déployer en préproduction','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur\n* Les tests de bout en bout couvrent le cas nominal\n\nLié à WEB-6.',NULL,'10001','3','3',16,4,16,NULL,'2025-11-10 18:24:57.000','2025-12-21 15:22:48.000','2025-12-03 11:05:56.000',NULL,NULL,NULL,NULL,0,'0|i02ha8:',NULL);
INSERT INTO `issues` VALUES (10043,10003,6,'3',NULL,'Augmenter l\'espace disque sur srv-web-01','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}\n\nLié à OPS-3.',NULL,'1','3',NULL,19,7,19,NULL,'2025-11-11 14:57:31.000','2025-11-29 12:38:29.000',NULL,NULL,118800,61200,57600,0,'0|i00pog:',NULL);
INSERT INTO `issues` VALUES (10044,10001,12,'5',10023,'Revue de code','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Les droits existants sont respectés\n\n{code:sql}\nSELECT id, statut FROM commande\nWHERE cree_le > now() - interval \'7 days\';\n{code}',NULL,'10001','3','10000',10,4,10,NULL,'2025-11-14 09:15:47.000','2025-12-11 15:48:07.000','2025-12-11 15:48:07.000',NULL,NULL,NULL,NULL,0,'0|i02hog:',NULL);
INSERT INTO `issues` VALUES (10045,10000,25,'10001',NULL,'En tant que administrateur, je veux partager un panier','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Un message clair s\'affiche en cas d\'erreur\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}',NULL,'10001','5','10000',13,9,13,NULL,'2025-11-15 15:11:50.000','2026-09-24 10:09:55.000','2025-12-04 16:51:29.000',NULL,NULL,NULL,NULL,0,'0|i038xs:',10003);
INSERT INTO `issues` VALUES (10046,10000,26,'1',NULL,'Le formulaire d\'inscription ne respecte pas les droits d\'accès','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\n*Étapes pour reproduire :*\n# Changer la langue en anglais\n# Rafraîchir la page\n# Ajouter deux articles au panier\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.',NULL,'10002','4',NULL,5,7,5,1,'2025-11-16 09:35:59.000','2025-12-09 16:08:16.000',NULL,'2025-12-03',NULL,NULL,NULL,0,'0|i01iww:',NULL);
INSERT INTO `issues` VALUES (10047,10003,7,'1',NULL,'Mettre à jour PostgreSQL sur proxy-01','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\n*Étapes pour reproduire :*\n# Se connecter avec un compte client\n# Ouvrir le tableau de bord\n# Ajouter deux articles au panier\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n[~julien.roux] peux-tu valider ?','Firefox 131 / macOS 15','6','4','5',22,9,22,NULL,'2025-11-16 12:48:10.000','2025-12-27 16:59:34.000','2025-11-17 16:49:29.000',NULL,NULL,NULL,NULL,0,'0|i03280:',NULL);
INSERT INTO `issues` VALUES (10048,10003,8,'4',NULL,'Mettre à jour PostgreSQL sur srv-web-01','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur',NULL,'3','1',NULL,15,22,15,NULL,'2025-11-17 14:43:33.000','2026-04-12 16:35:10.000',NULL,'2025-12-26',NULL,NULL,NULL,0,'0|i0016o:',NULL);
INSERT INTO `issues` VALUES (10049,10000,27,'10001',NULL,'En tant que client, je veux recevoir une alerte de stock','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur',NULL,'10001','3','3',4,9,4,NULL,'2025-11-18 17:52:18.000','2025-12-14 18:47:00.000','2025-11-19 09:52:07.000',NULL,133200,133200,NULL,0,'0|i02i2o:',NULL);
INSERT INTO `issues` VALUES (10050,10000,28,'10000',NULL,'Internationalisation','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur',NULL,'10001','3','10000',20,NULL,20,NULL,'2025-11-19 15:31:19.000','2025-11-21 09:09:58.000','2025-11-19 18:01:19.000',NULL,79200,79200,NULL,0,'0|i02igw:',NULL);
INSERT INTO `issues` VALUES (10051,10003,9,'3',NULL,'Mettre à jour PostgreSQL sur srv-web-01',NULL,NULL,'3','3',NULL,2,15,2,NULL,'2025-11-19 16:59:52.000','2025-12-11 17:43:55.000',NULL,NULL,64800,64800,NULL,0,'0|i00q2o:',NULL);
INSERT INTO `issues` VALUES (10052,10001,13,'10001',NULL,'En tant que administrateur, je veux réinitialiser mon mot de passe','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Un message clair s\'affiche en cas d\'erreur\n* Les droits existants sont respectés',NULL,'3','4',NULL,11,7,11,NULL,'2025-11-20 07:01:20.000','2025-12-14 12:08:00.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01jb4:',NULL);
INSERT INTO `issues` VALUES (10053,10003,10,'4',NULL,'Renouveler le certificat TLS de srv-batch-01','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}',NULL,'5','3','4',9,NULL,9,NULL,'2025-11-21 15:15:29.000','2025-11-26 07:22:11.000','2025-11-26 07:22:11.000',NULL,NULL,NULL,NULL,0,'0|i02iv4:',NULL);
INSERT INTO `issues` VALUES (10054,10000,29,'1',NULL,'Le formulaire d\'inscription se vide au rafraîchissement','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\n*Étapes pour reproduire :*\n# Changer la langue en anglais\n# Ajouter deux articles au panier\n# Lancer l\'export\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}',NULL,'10000','4',NULL,24,16,24,NULL,'2025-11-22 10:28:28.000','2026-01-06 18:52:58.000',NULL,'2025-12-11',57600,57600,NULL,0,'0|i01jpc:',10003);
INSERT INTO `issues` VALUES (10055,10001,14,'10001',NULL,'En tant que administrateur, je veux recevoir une alerte de stock',NULL,NULL,'3','2',NULL,6,17,6,NULL,'2025-11-22 13:19:53.000','2026-01-01 07:26:23.000',NULL,NULL,NULL,NULL,NULL,0,'0|i006ps:',NULL);
INSERT INTO `issues` VALUES (10056,10000,30,'1',NULL,'L\'API de facturation ne respecte pas les droits d\'accès','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\n*Étapes pour reproduire :*\n# Ajouter deux articles au panier\n# Ouvrir le tableau de bord\n# Lancer l\'export\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\nVoir [la maquette|https://maquettes.exemple.fr/panier]. Lié à DEMO-23. [~lucas.bernard] peux-tu valider ?','Chrome 129 / Windows 11','10000','3',NULL,16,6,16,NULL,'2025-11-26 07:44:18.000','2025-12-19 15:16:06.000',NULL,'2026-01-01',68400,0,88200,0,'0|i00qgw:',NULL);
INSERT INTO `issues` VALUES (10057,10003,11,'3',NULL,'Ouvrir le flux réseau entre srv-db-01 et le VPN','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Le résultat est identique sur mobile et sur ordinateur',NULL,'1','4',NULL,9,22,9,NULL,'2025-11-29 11:26:47.000','2026-01-13 11:28:57.000',NULL,NULL,NULL,NULL,6300,0,'0|i01k3k:',NULL);
INSERT INTO `issues` VALUES (10058,10000,31,'5',10041,'Mettre à jour la documentation','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur\n\nVoir [la maquette|https://maquettes.exemple.fr/panier]. Lié à DEMO-22.',NULL,'10000','3',NULL,7,15,7,NULL,'2025-11-29 12:08:29.000','2025-12-26 13:17:53.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00qv4:',NULL);
INSERT INTO `issues` VALUES (10059,10000,32,'1',NULL,'L\'export PDF des factures s\'affiche mal sur mobile','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\n*Étapes pour reproduire :*\n# Ouvrir le tableau de bord\n# Rafraîchir la page\n# Ajouter deux articles au panier\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412].',NULL,'3','2',NULL,16,NULL,16,2,'2025-11-29 13:43:35.000','2025-12-11 18:47:11.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00740:',NULL);
INSERT INTO `issues` VALUES (10060,10000,33,'3',NULL,'Ajouter le tableau des commandes','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Un message clair s\'affiche en cas d\'erreur\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412].',NULL,'3','3',NULL,6,NULL,6,NULL,'2025-12-01 16:57:04.000','2025-12-22 17:00:03.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00r9c:',NULL);
INSERT INTO `issues` VALUES (10061,10000,34,'10001',NULL,'En tant que client, je veux réinitialiser mon mot de passe','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur\n\nLié à DEMO-1.',NULL,'10001','2','10000',3,4,3,NULL,'2025-12-02 16:04:04.000','2026-01-27 18:56:18.000','2025-12-22 16:18:28.000','2026-01-16',36000,36000,NULL,0,'0|i02328:',NULL);
INSERT INTO `issues` VALUES (10062,10000,35,'10001',NULL,'En tant que commercial, je veux exporter mes factures','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}\n\n!maquette.jpg!',NULL,'10002','1',NULL,17,4,17,NULL,'2025-12-03 15:10:02.000','2025-12-25 11:07:54.000',NULL,NULL,NULL,NULL,15300,0,'0|i001kw:',10050);
INSERT INTO `issues` VALUES (10063,10000,36,'1',NULL,'Le calendrier des livraisons ignore les caractères accentués','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\n*Étapes pour reproduire :*\n# Lancer l\'export\n# Ajouter deux articles au panier\n# Changer la langue en anglais\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}\n\n[~jeanne.dupont] peux-tu valider ?',NULL,'10002','2',NULL,2,4,2,NULL,'2025-12-03 16:31:05.000','2025-12-21 07:28:46.000',NULL,NULL,140400,140400,NULL,0,'0|i007i8:',NULL);
INSERT INTO `issues` VALUES (10064,10001,15,'10001',NULL,'En tant que client, je veux partager un panier',NULL,NULL,'10001','3','2',6,16,6,NULL,'2025-12-04 13:57:06.000','2025-12-21 16:25:15.000','2025-12-19 18:17:27.000',NULL,3600,0,29700,0,'0|i02j9c:',NULL);
INSERT INTO `issues` VALUES (10065,10003,12,'3',NULL,'Augmenter l\'espace disque sur proxy-01','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* La page répond en moins d\'une seconde',NULL,'6','2','1',9,NULL,1,NULL,'2025-12-06 13:15:52.000','2025-12-17 11:07:25.000','2025-12-12 16:43:04.000',NULL,50400,50400,NULL,0,'0|i023gg:',NULL);
INSERT INTO `issues` VALUES (10066,10003,13,'3',NULL,'Superviser la file de messages sur srv-db-01','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}\n\n[~thomas.moreau] peux-tu valider ?',NULL,'6','4','2',11,23,11,NULL,'2025-12-07 11:42:01.000','2026-09-26 07:45:18.000','2025-12-08 16:08:21.000',NULL,NULL,NULL,NULL,0,'0|i032m8:',NULL);
INSERT INTO `issues` VALUES (10067,10003,14,'3',NULL,'Redémarrage inopiné de proxy-01','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* La page répond en moins d\'une seconde',NULL,'6','2','1',11,23,11,NULL,'2025-12-08 08:57:49.000','2026-01-01 11:52:16.000','2025-12-10 13:19:38.000',NULL,NULL,NULL,NULL,0,'0|i023uo:',NULL);
INSERT INTO `issues` VALUES (10068,10000,37,'10001',NULL,'En tant que commercial, je veux partager un panier','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n* Un message clair s\'affiche en cas d\'erreur\n\n{code:sql}\nSELECT id, statut FROM commande\nWHERE cree_le > now() - interval \'7 days\';\n{code}\n\n[~thomas.moreau] peux-tu valider ?\n\n!capture.png!',NULL,'10000','4',NULL,10,7,10,NULL,'2025-12-08 11:44:14.000','2026-03-30 16:24:04.000',NULL,NULL,18000,18000,NULL,0,'0|i01khs:',10050);
INSERT INTO `issues` VALUES (10069,10000,38,'10001',NULL,'En tant que client, je veux recevoir une alerte de stock','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur\n\n[~camille.petit] peux-tu valider ?',NULL,'10001','1','10000',7,4,7,NULL,'2025-12-09 10:22:12.000','2026-01-31 09:22:21.000','2025-12-31 11:10:23.000',NULL,NULL,NULL,NULL,0,'0|i01xj4:',NULL);
INSERT INTO `issues` VALUES (10070,10003,15,'5',10024,'Corriger les retours de recette','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Les tests de bout en bout couvrent le cas nominal\n* La page répond en moins d\'une seconde\n\nLié à OPS-4.',NULL,'5','1','2',23,11,1,NULL,'2025-12-09 12:17:57.000','2025-12-22 16:44:49.000','2025-12-19 15:00:46.000',NULL,NULL,NULL,NULL,0,'0|i01xxc:',NULL);
INSERT INTO `issues` VALUES (10071,10002,4,'3',NULL,'Mettre à jour la grille salariale','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n\nLié à RH-2.',NULL,'1','3',NULL,10,8,10,NULL,'2025-12-09 14:52:07.000','2025-12-28 11:24:30.000',NULL,NULL,28800,28800,NULL,0,'0|i00rnk:',NULL);
INSERT INTO `issues` VALUES (10072,10000,39,'10001',NULL,'En tant que client, je veux réinitialiser mon mot de passe','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Un message clair s\'affiche en cas d\'erreur',NULL,'10001','3','2',7,2,7,NULL,'2025-12-09 18:23:09.000','2026-02-17 07:16:24.000','2026-01-02 14:14:36.000',NULL,NULL,NULL,NULL,0,'0|i02jnk:',10003);
INSERT INTO `issues` VALUES (10073,10001,16,'5',10052,'Revue de code','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Le résultat est identique sur mobile et sur ordinateur\n* Un message clair s\'affiche en cas d\'erreur',NULL,'10001','1','10000',4,NULL,4,NULL,'2025-12-12 18:23:25.000','2026-01-21 14:35:57.000','2025-12-14 10:32:46.000',NULL,NULL,NULL,NULL,0,'0|i01ybk:',NULL);
INSERT INTO `issues` VALUES (10074,10000,40,'10001',NULL,'En tant que agent du support, je veux suivre ma livraison','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* La page répond en moins d\'une seconde\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412]. [~antoine.lefebvre] peux-tu valider ?',NULL,'10001','3','10000',17,13,17,NULL,'2025-12-13 18:07:50.000','2026-01-17 07:56:40.000','2025-12-24 07:52:13.000',NULL,NULL,NULL,NULL,0,'0|i02k1s:',NULL);
INSERT INTO `issues` VALUES (10075,10000,41,'1',NULL,'Le formulaire d\'inscription perd les données saisies',NULL,'Safari 18 / iOS 18','10000','3',NULL,17,3,17,NULL,'2025-12-14 18:25:28.000','2025-12-29 13:32:46.000',NULL,NULL,NULL,NULL,18000,0,'0|i00s1s:',10003);
INSERT INTO `issues` VALUES (10076,10001,17,'5',10009,'Préparer la recette','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Le résultat est identique sur mobile et sur ordinateur\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}\n\nLié à WEB-16. [~lucas.bernard] peux-tu valider ?\n\n!capture.png!',NULL,'3','2',NULL,3,2,1,NULL,'2025-12-15 15:28:10.000','2026-01-03 15:11:25.000',NULL,NULL,57600,57600,NULL,0,'0|i007wg:',NULL);
INSERT INTO `issues` VALUES (10077,10001,18,'10001',NULL,'En tant que agent du support, je veux suivre ma livraison','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Un message clair s\'affiche en cas d\'erreur\n\nVoir [la documentation interne|https://wiki.exemple.fr/exploitation/sauvegardes]. Lié à WEB-10.',NULL,'3','4',NULL,16,NULL,16,NULL,'2025-12-16 07:00:41.000','2026-04-05 17:33:57.000',NULL,NULL,93600,87300,6300,0,'0|i01kw0:',NULL);
INSERT INTO `issues` VALUES (10078,10002,5,'3',NULL,'Valider les congés d\'été','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Les tests de bout en bout couvrent le cas nominal\n* La page répond en moins d\'une seconde',NULL,'3','3',NULL,8,14,8,NULL,'2025-12-16 10:27:03.000','2025-12-31 17:29:23.000',NULL,NULL,NULL,NULL,48600,0,'0|i00sg0:',NULL);
INSERT INTO `issues` VALUES (10079,10002,6,'3',NULL,'Mettre à jour la grille salariale','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur\n\n!capture.png!',NULL,'3','4',NULL,8,14,8,NULL,'2025-12-19 12:34:16.000','2026-01-05 09:29:40.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01la8:',NULL);
INSERT INTO `issues` VALUES (10080,10000,42,'10000',NULL,'Conformité RGPD','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Un message clair s\'affiche en cas d\'erreur\n* Le résultat est identique sur mobile et sur ordinateur\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}\n\nVoir [la documentation interne|https://wiki.exemple.fr/exploitation/sauvegardes].\n\n!capture.png!',NULL,'10000','4',NULL,16,7,16,NULL,'2025-12-20 10:25:09.000','2026-02-07 14:49:07.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01log:',NULL);
INSERT INTO `issues` VALUES (10081,10001,19,'10001',NULL,'En tant que administrateur, je veux suivre ma livraison','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* La page répond en moins d\'une seconde\n\n{code:sql}\nSELECT id, statut FROM commande\nWHERE cree_le > now() - interval \'7 days\';\n{code}',NULL,'10000','3',NULL,9,17,9,NULL,'2025-12-23 15:31:31.000','2026-01-30 07:15:24.000',NULL,NULL,108000,77400,30600,0,'0|i00su8:',NULL);
INSERT INTO `issues` VALUES (10082,10002,7,'3',NULL,'Préparer la visite médicale de M. Roux','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Les tests de bout en bout couvrent le cas nominal\n\n[~hugo.robert] peux-tu valider ?',NULL,'5','3','1',10,14,10,NULL,'2025-12-26 16:57:51.000','2026-01-02 12:42:21.000','2026-01-01 10:43:19.000','2026-02-14',NULL,NULL,NULL,0,'0|i02kg0:',NULL);
INSERT INTO `issues` VALUES (10083,10001,20,'10001',NULL,'En tant que client, je veux exporter mes factures','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Les tests de bout en bout couvrent le cas nominal\n* Un message clair s\'affiche en cas d\'erreur',NULL,'10001','4','10000',7,2,7,NULL,'2025-12-27 08:23:01.000','2026-01-15 16:18:05.000','2025-12-31 18:24:50.000',NULL,NULL,NULL,NULL,0,'0|i0330g:',NULL);
INSERT INTO `issues` VALUES (10084,10001,21,'10001',NULL,'En tant que administrateur, je veux filtrer les commandes par statut',NULL,NULL,'10000','3',NULL,19,7,19,NULL,'2025-12-28 08:40:48.000','2026-01-24 08:39:30.000',NULL,NULL,57600,57600,NULL,0,'0|i00t8g:',NULL);
INSERT INTO `issues` VALUES (10085,10000,43,'3',NULL,'Tester les tests de bout en bout','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Les tests de bout en bout couvrent le cas nominal\n\nLié à DEMO-9. [~maxime.bertrand] peux-tu valider ?\n\n!maquette.jpg!\n\n!capture.png!',NULL,'10000','2',NULL,9,3,9,NULL,'2025-12-28 16:23:42.000','2026-01-10 16:32:17.000',NULL,'2026-02-25',36000,0,63900,0,'0|i008ao:',10080);
INSERT INTO `issues` VALUES (10086,10003,16,'5',10065,'Rédiger les tests unitaires','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les droits existants sont respectés\n* Le résultat est identique sur mobile et sur ordinateur\n\nVoir [la documentation interne|https://wiki.exemple.fr/exploitation/sauvegardes]. [~antoine.lefebvre] peux-tu valider ?\n\n!capture.png!',NULL,'5','2','1',7,21,7,NULL,'2025-12-31 18:48:00.000','2026-01-27 18:57:24.000','2026-01-02 13:41:39.000',NULL,NULL,NULL,61200,0,'0|i0248w:',NULL);
INSERT INTO `issues` VALUES (10087,10000,44,'10001',NULL,'En tant que administrateur, je veux réinitialiser mon mot de passe','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur',NULL,'10000','2',NULL,20,11,20,NULL,'2026-01-01 17:17:28.000','2026-02-02 08:07:11.000',NULL,'2026-01-15',NULL,NULL,NULL,0,'0|i008ow:',10050);
INSERT INTO `issues` VALUES (10088,10001,22,'10000',NULL,'Refonte du tunnel de commande','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Un message clair s\'affiche en cas d\'erreur',NULL,'3','3',NULL,17,9,17,NULL,'2026-01-03 14:29:42.000','2026-09-24 16:13:12.000',NULL,NULL,14400,14400,NULL,0,'0|i00tmo:',NULL);
INSERT INTO `issues` VALUES (10089,10000,45,'10001',NULL,'En tant que administrateur, je veux recevoir une alerte de stock','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* La page répond en moins d\'une seconde\n\n[~camille.petit] peux-tu valider ?',NULL,'10001','2','10000',10,6,10,NULL,'2026-01-04 08:50:50.000','2026-01-25 07:18:11.000','2026-01-25 07:18:11.000',NULL,NULL,NULL,12600,0,'0|i024n4:',10050);
INSERT INTO `issues` VALUES (10090,10000,46,'1',NULL,'L\'import CSV se vide au rafraîchissement','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\n*Étapes pour reproduire :*\n# Se connecter avec un compte client\n# Changer la langue en anglais\n# Ouvrir le tableau de bord\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\nLié à DEMO-26.',NULL,'10001','3','10000',3,NULL,3,NULL,'2026-01-05 18:20:56.000','2026-01-27 12:08:52.000','2026-01-27 12:08:52.000',NULL,115200,115200,NULL,0,'0|i02ku8:',NULL);
INSERT INTO `issues` VALUES (10091,10000,47,'10001',NULL,'En tant que commercial, je veux partager un panier','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* La page répond en moins d\'une seconde',NULL,'3','3',NULL,11,NULL,11,2,'2026-01-06 10:45:28.000','2026-01-18 11:25:52.000',NULL,NULL,NULL,NULL,21600,0,'0|i00u0w:',NULL);
INSERT INTO `issues` VALUES (10092,10000,48,'10001',NULL,'En tant que client, je veux filtrer les commandes par statut','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde',NULL,'10000','3',NULL,11,4,11,NULL,'2026-01-09 16:27:55.000','2026-01-19 17:42:00.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00uf4:',NULL);
INSERT INTO `issues` VALUES (10093,10000,49,'10001',NULL,'En tant que administrateur, je veux réinitialiser mon mot de passe','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Les droits existants sont respectés',NULL,'3','4',NULL,7,5,7,NULL,'2026-01-17 10:51:13.000','2026-02-16 16:44:25.000',NULL,NULL,100800,100800,NULL,0,'0|i01m2o:',NULL);
INSERT INTO `issues` VALUES (10094,10000,50,'1',NULL,'La synchronisation mobile plante sur Safari 18','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\n*Étapes pour reproduire :*\n# Rafraîchir la page\n# Se connecter avec un compte client\n# Ajouter deux articles au panier\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}',NULL,'10000','3',NULL,15,5,15,NULL,'2026-01-21 18:23:27.000','2026-02-21 16:43:40.000',NULL,'2026-02-06',NULL,NULL,69300,0,'0|i00utc:',NULL);
INSERT INTO `issues` VALUES (10095,10002,8,'3',NULL,'Préparer l\'entretien annuel de M. Roux','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}\n\nLié à RH-7.',NULL,'5','4','5',8,14,8,NULL,'2026-01-23 08:30:51.000','2026-02-17 16:33:58.000','2026-02-10 14:03:52.000','2026-02-09',129600,129600,NULL,0,'0|i033eo:',NULL);
INSERT INTO `issues` VALUES (10096,10003,17,'1',NULL,'Sauvegarde nocturne en échec sur srv-db-02','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\n*Étapes pour reproduire :*\n# Lancer l\'export\n# Changer la langue en anglais\n# Se connecter avec un compte client\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}','Firefox 131 / macOS 15','5','2','1',7,15,7,NULL,'2026-01-24 14:59:26.000','2026-02-16 18:59:54.000','2026-02-02 10:29:07.000','2026-02-14',93600,19800,73800,0,'0|i0251c:',NULL);
INSERT INTO `issues` VALUES (10097,10002,9,'3',NULL,'Préparer l\'entretien annuel de Mme Petit','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412].',NULL,'5','3','4',10,14,10,NULL,'2026-01-26 07:13:20.000','2026-02-19 10:52:01.000','2026-02-11 13:12:07.000',NULL,50400,14400,36000,0,'0|i02l8g:',NULL);
INSERT INTO `issues` VALUES (10098,10002,10,'3',NULL,'Mettre à jour la grille salariale','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Le résultat est identique sur mobile et sur ordinateur\n* Les tests de bout en bout couvrent le cas nominal\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}\n\n[~ines.david] peux-tu valider ?',NULL,'3','1',NULL,8,8,8,NULL,'2026-01-26 09:04:28.000','2026-01-30 16:03:43.000',NULL,NULL,NULL,NULL,NULL,0,'0|i001z4:',NULL);
INSERT INTO `issues` VALUES (10099,10003,18,'4',NULL,'Sauvegarde nocturne en échec sur srv-batch-01','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Un message clair s\'affiche en cas d\'erreur',NULL,'5','4','1',15,NULL,15,NULL,'2026-01-27 18:54:36.000','2026-02-07 07:52:29.000','2026-02-07 07:52:29.000','2026-03-03',NULL,NULL,NULL,0,'0|i033sw:',NULL);
INSERT INTO `issues` VALUES (10100,10001,23,'10001',NULL,'En tant que commercial, je veux voir l\'historique de mes demandes',NULL,NULL,'10001','3','10000',12,15,12,NULL,'2026-01-28 12:19:39.000','2026-02-13 12:47:25.000','2026-02-06 12:01:50.000',NULL,10800,10800,NULL,0,'0|i02lmo:',10088);
INSERT INTO `issues` VALUES (10101,10001,24,'5',10037,'Mettre à jour la documentation','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Un message clair s\'affiche en cas d\'erreur\n* Les droits existants sont respectés\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}\n\n[~julien.roux] peux-tu valider ?',NULL,'10001','1','10000',9,6,9,NULL,'2026-01-28 13:50:13.000','2026-02-25 16:05:51.000','2026-02-10 07:59:05.000',NULL,14400,14400,NULL,0,'0|i01yps:',NULL);
INSERT INTO `issues` VALUES (10102,10003,19,'4',NULL,'Ouvrir le flux réseau entre srv-db-01 et le VPN','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Les droits existants sont respectés\n* La page répond en moins d\'une seconde\n\n!capture.png!',NULL,'1','5',NULL,21,22,21,NULL,'2026-01-30 17:42:53.000','2026-02-19 11:40:01.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01udc:',NULL);
INSERT INTO `issues` VALUES (10103,10001,25,'10001',NULL,'En tant que commercial, je veux filtrer les commandes par statut','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur\n\n[~maxime.bertrand] peux-tu valider ?',NULL,'10000','4',NULL,6,NULL,6,NULL,'2026-02-02 07:54:25.000','2026-02-04 16:46:10.000',NULL,'2026-02-22',NULL,NULL,NULL,0,'0|i01mgw:',NULL);
INSERT INTO `issues` VALUES (10104,10001,26,'3',NULL,'Tester la gestion des rôles','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}',NULL,'10000','3',NULL,11,16,11,NULL,'2026-02-02 08:25:36.000','2026-06-23 17:29:52.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00v7k:',10088);
INSERT INTO `issues` VALUES (10105,10000,51,'10000',NULL,'Conformité RGPD','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412].',NULL,'10000','3',NULL,11,11,11,NULL,'2026-02-03 10:07:48.000','2026-03-09 15:27:25.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00vls:',NULL);
INSERT INTO `issues` VALUES (10106,10002,11,'5',10098,'Revue de code','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Le résultat est identique sur mobile et sur ordinateur\n\nVoir [la maquette|https://maquettes.exemple.fr/panier]. Lié à RH-10.',NULL,'3','3',NULL,10,10,10,NULL,'2026-02-03 11:17:56.000','2026-02-07 16:18:51.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00w00:',NULL);
INSERT INTO `issues` VALUES (10107,10001,27,'10001',NULL,'En tant que agent du support, je veux voir l\'historique de mes demandes',NULL,NULL,'10000','3',NULL,4,13,4,NULL,'2026-02-03 12:49:16.000','2026-05-08 12:58:03.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00we8:',10088);
INSERT INTO `issues` VALUES (10108,10000,52,'5',10087,'Rédiger les tests unitaires','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les droits existants sont respectés\n\nLié à DEMO-12.',NULL,'10000','3',NULL,10,3,10,NULL,'2026-02-03 15:28:00.000','2026-02-20 09:58:16.000',NULL,NULL,144000,144000,NULL,0,'0|i00wsg:',NULL);
INSERT INTO `issues` VALUES (10109,10000,53,'10001',NULL,'En tant que client, je veux partager un panier','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur\n\n[~jeanne.dupont] peux-tu valider ?',NULL,'10002','2',NULL,16,11,16,NULL,'2026-02-07 13:08:33.000','2026-03-20 10:29:23.000',NULL,NULL,68400,57600,10800,0,'0|i00934:',10003);
INSERT INTO `issues` VALUES (10110,10000,54,'3',NULL,'Automatiser la page de profil','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Les droits existants sont respectés\n* La page répond en moins d\'une seconde\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412]. [~lucas.bernard] peux-tu valider ?',NULL,'3','3',NULL,13,NULL,13,NULL,'2026-02-08 08:13:21.000','2026-02-16 13:50:22.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00x6o:',NULL);
INSERT INTO `issues` VALUES (10111,10001,28,'1',NULL,'L\'API de facturation envoie deux fois le même message','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\n*Étapes pour reproduire :*\n# Lancer l\'export\n# Ajouter deux articles au panier\n# Changer la langue en anglais\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.','Chrome 129 / Windows 11','10001','4','10000',10,2,10,NULL,'2026-02-08 13:27:59.000','2026-02-18 09:12:35.000','2026-02-18 09:12:35.000',NULL,NULL,NULL,NULL,0,'0|i03474:',NULL);
INSERT INTO `issues` VALUES (10112,10000,55,'10001',NULL,'En tant que commercial, je veux exporter mes factures',NULL,NULL,'10000','3',NULL,3,5,3,NULL,'2026-02-10 07:22:33.000','2026-02-16 12:58:10.000',NULL,NULL,36000,36000,NULL,0,'0|i00xkw:',NULL);
INSERT INTO `issues` VALUES (10113,10003,20,'4',NULL,'Superviser la file de messages sur srv-batch-01','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n\n{code:sql}\nSELECT id, statut FROM commande\nWHERE cree_le > now() - interval \'7 days\';\n{code}\n\nLié à OPS-19.',NULL,'6','3','3',11,NULL,11,NULL,'2026-02-10 15:37:13.000','2026-02-27 18:45:48.000','2026-02-17 18:58:40.000',NULL,129600,85500,44100,0,'0|i02m0w:',NULL);
INSERT INTO `issues` VALUES (10114,10002,12,'4',NULL,'Préparer l\'entretien annuel de M. Girard','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Le résultat est identique sur mobile et sur ordinateur',NULL,'6','3','1',10,NULL,10,NULL,'2026-02-13 13:20:27.000','2026-03-04 07:48:17.000','2026-02-24 17:19:03.000',NULL,97200,97200,NULL,0,'0|i02mf4:',NULL);
INSERT INTO `issues` VALUES (10115,10002,13,'3',NULL,'Valider les congés d\'été',NULL,NULL,'1','3',NULL,14,14,14,NULL,'2026-02-13 14:07:25.000','2026-03-05 11:26:05.000',NULL,NULL,72000,72000,NULL,0,'0|i00xz4:',NULL);
INSERT INTO `issues` VALUES (10116,10003,21,'1',NULL,'Mettre à jour PostgreSQL sur srv-web-01',NULL,'Safari 18 / iOS 18','6','2','1',15,7,15,NULL,'2026-02-13 17:25:32.000','2026-03-09 09:18:21.000','2026-02-25 08:42:44.000','2026-03-12',NULL,NULL,NULL,0,'0|i025fk:',NULL);
INSERT INTO `issues` VALUES (10117,10001,29,'1',NULL,'Le formulaire d\'inscription ignore les caractères accentués','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\n*Étapes pour reproduire :*\n# Changer la langue en anglais\n# Se connecter avec un compte client\n# Ouvrir le tableau de bord\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.','Recette — https://recette.exemple.fr','10000','4',NULL,15,13,15,NULL,'2026-02-16 11:19:56.000','2026-06-03 17:40:49.000',NULL,'2026-03-06',NULL,NULL,NULL,0,'0|i01mv4:',NULL);
INSERT INTO `issues` VALUES (10118,10003,22,'4',NULL,'Mettre à jour PostgreSQL sur srv-db-01','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Le résultat est identique sur mobile et sur ordinateur\n* Les tests de bout en bout couvrent le cas nominal\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}',NULL,'1','4',NULL,12,7,12,NULL,'2026-02-18 18:35:37.000','2026-03-31 12:53:52.000',NULL,'2026-03-22',NULL,NULL,NULL,0,'0|i01n9c:',NULL);
INSERT INTO `issues` VALUES (10119,10000,56,'1',NULL,'Le module de paiement se vide au rafraîchissement','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\n*Étapes pour reproduire :*\n# Ouvrir le tableau de bord\n# Ajouter deux articles au panier\n# Lancer l\'export\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n[~nicolas.laurent] peux-tu valider ?','Safari 18 / iOS 18','10001','4','10000',2,15,2,NULL,'2026-02-19 08:03:22.000','2026-03-13 14:40:43.000','2026-03-04 13:44:39.000',NULL,46800,46800,NULL,0,'0|i034lc:',NULL);
INSERT INTO `issues` VALUES (10120,10001,30,'3',NULL,'Tester la page d\'accueil','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Un message clair s\'affiche en cas d\'erreur\n\nVoir [la maquette|https://maquettes.exemple.fr/panier].',NULL,'10001','3','10000',5,9,5,NULL,'2026-02-22 17:43:26.000','2026-03-06 08:32:36.000','2026-03-06 08:32:36.000','2026-04-02',133200,133200,NULL,0,'0|i02mtc:',10088);
INSERT INTO `issues` VALUES (10121,10003,23,'4',NULL,'Sauvegarde nocturne en échec sur srv-web-02',NULL,NULL,'6','3','5',7,7,7,NULL,'2026-02-23 11:51:06.000','2026-03-20 18:51:15.000','2026-03-02 09:06:43.000',NULL,93600,93600,NULL,0,'0|i02n7k:',NULL);
INSERT INTO `issues` VALUES (10122,10001,31,'10001',NULL,'En tant que administrateur, je veux recevoir une alerte de stock',NULL,NULL,'10000','3',NULL,17,5,17,NULL,'2026-02-23 13:25:54.000','2026-03-10 16:51:22.000',NULL,'2026-04-23',NULL,NULL,NULL,0,'0|i00ydc:',10088);
INSERT INTO `issues` VALUES (10123,10003,24,'3',NULL,'Superviser la file de messages sur srv-web-02','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* La page répond en moins d\'une seconde',NULL,'5','5','1',23,15,23,NULL,'2026-02-23 16:31:42.000','2026-02-28 08:20:39.000','2026-02-26 18:28:01.000',NULL,NULL,NULL,24300,0,'0|i039c0:',NULL);
INSERT INTO `issues` VALUES (10124,10001,32,'3',NULL,'Automatiser les tests de bout en bout','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur\n\nLié à WEB-14. [~julien.roux] peux-tu valider ?',NULL,'10000','4',NULL,11,15,11,NULL,'2026-02-23 18:45:27.000','2026-03-19 10:14:15.000',NULL,NULL,115200,77400,37800,0,'0|i01nnk:',NULL);
INSERT INTO `issues` VALUES (10125,10003,25,'1',NULL,'Mettre à jour PostgreSQL sur srv-web-02',NULL,NULL,'5','3','1',2,NULL,2,NULL,'2026-02-24 10:10:11.000','2026-03-13 16:23:00.000','2026-03-13 16:23:00.000',NULL,NULL,NULL,NULL,0,'0|i02nls:',NULL);
INSERT INTO `issues` VALUES (10126,10000,57,'10001',NULL,'En tant que commercial, je veux suivre ma livraison','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Les droits existants sont respectés\n* Le résultat est identique sur mobile et sur ordinateur',NULL,'10002','3',NULL,12,4,12,NULL,'2026-02-24 16:03:31.000','2026-03-20 18:40:43.000',NULL,NULL,NULL,NULL,68400,0,'0|i00yrk:',10003);
INSERT INTO `issues` VALUES (10127,10001,33,'1',NULL,'Le panier affiche une date décalée d\'une heure','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\n*Étapes pour reproduire :*\n# Rafraîchir la page\n# Changer la langue en anglais\n# Lancer l\'export\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}',NULL,'10001','3','10000',6,9,6,NULL,'2026-02-26 16:39:24.000','2026-03-28 10:55:37.000','2026-03-13 09:53:23.000','2026-03-13',14400,14400,NULL,0,'0|i02o00:',NULL);
INSERT INTO `issues` VALUES (10128,10001,34,'10001',NULL,'En tant que client, je veux recevoir une alerte de stock','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n\n[~lucas.bernard] peux-tu valider ?',NULL,'10000','1',NULL,19,NULL,19,NULL,'2026-02-28 08:25:20.000','2026-04-12 16:36:15.000',NULL,NULL,75600,75600,NULL,0,'0|i002dc:',NULL);
INSERT INTO `issues` VALUES (10129,10000,58,'1',NULL,'La synchronisation mobile s\'affiche mal sur mobile','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\n*Étapes pour reproduire :*\n# Ajouter deux articles au panier\n# Rafraîchir la page\n# Ouvrir le tableau de bord\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}\n\nVoir [la documentation interne|https://wiki.exemple.fr/exploitation/sauvegardes].','Recette — https://recette.exemple.fr','10000','2',NULL,18,NULL,18,NULL,'2026-03-01 08:57:09.000','2026-03-16 08:02:50.000',NULL,NULL,NULL,NULL,NULL,0,'0|i009hc:',NULL);
INSERT INTO `issues` VALUES (10130,10000,59,'5',10002,'Déployer en préproduction','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les droits existants sont respectés\n\nLié à DEMO-38.',NULL,'3','3',NULL,7,3,7,NULL,'2026-03-02 08:04:29.000','2026-03-13 10:16:38.000',NULL,NULL,75600,74700,900,0,'0|i00z5s:',NULL);
INSERT INTO `issues` VALUES (10131,10001,35,'1',NULL,'Le formulaire d\'inscription renvoie une erreur 500','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\n*Étapes pour reproduire :*\n# Rafraîchir la page\n# Lancer l\'export\n# Ajouter deux articles au panier\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}',NULL,'3','3',NULL,4,5,1,NULL,'2026-03-03 08:19:42.000','2026-04-10 06:43:37.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00zk0:',NULL);
INSERT INTO `issues` VALUES (10132,10003,26,'3',NULL,'Superviser la file de messages sur proxy-01','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Les droits existants sont respectés\n\nLié à OPS-6.',NULL,'1','3',NULL,19,23,19,NULL,'2026-03-10 13:49:43.000','2026-04-06 13:05:35.000',NULL,NULL,14400,14400,NULL,0,'0|i00zy8:',NULL);
INSERT INTO `issues` VALUES (10133,10000,60,'10000',NULL,'Internationalisation',NULL,NULL,'10001','2','10000',6,13,6,NULL,'2026-03-11 08:39:24.000','2026-04-13 07:59:04.000','2026-03-31 09:01:43.000',NULL,NULL,NULL,NULL,0,'0|i025ts:',NULL);
INSERT INTO `issues` VALUES (10134,10000,61,'10001',NULL,'En tant que client, je veux exporter mes factures','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal',NULL,'10001','3','10000',5,16,5,NULL,'2026-03-12 16:50:47.000','2026-03-29 14:21:25.000','2026-03-12 16:55:23.000',NULL,NULL,NULL,NULL,0,'0|i02oe8:',NULL);
INSERT INTO `issues` VALUES (10135,10000,62,'10001',NULL,'En tant que agent du support, je veux filtrer les commandes par statut','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Un message clair s\'affiche en cas d\'erreur\n* La page répond en moins d\'une seconde',NULL,'10000','2',NULL,3,11,3,NULL,'2026-03-13 11:43:25.000','2026-04-01 12:41:24.000',NULL,NULL,75600,75600,NULL,0,'0|i009vk:',NULL);
INSERT INTO `issues` VALUES (10136,10001,36,'3',NULL,'Automatiser la gestion des rôles','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Les droits existants sont respectés\n* Les tests de bout en bout couvrent le cas nominal',NULL,'3','3',NULL,4,9,4,NULL,'2026-03-14 08:56:14.000','2026-03-27 11:05:02.000',NULL,NULL,NULL,NULL,NULL,0,'0|i010cg:',10088);
INSERT INTO `issues` VALUES (10137,10001,37,'3',NULL,'Tester le parcours d\'inscription','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n\nLié à WEB-21.',NULL,'10000','3',NULL,4,4,4,NULL,'2026-03-14 09:02:53.000','2026-03-14 09:02:53.000',NULL,NULL,NULL,NULL,NULL,0,'0|i010qo:',NULL);
INSERT INTO `issues` VALUES (10138,10000,63,'10001',NULL,'En tant que client, je veux filtrer les commandes par statut','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur\n\nVoir [la maquette|https://maquettes.exemple.fr/panier]. [~thomas.moreau] peux-tu valider ?',NULL,'10000','2',NULL,5,5,5,NULL,'2026-03-14 18:10:24.000','2026-07-27 08:40:34.000',NULL,NULL,144000,144000,NULL,0,'0|i00a9s:',10003);
INSERT INTO `issues` VALUES (10139,10000,64,'10001',NULL,'En tant que commercial, je veux voir l\'historique de mes demandes','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Un message clair s\'affiche en cas d\'erreur\n\n[~thomas.moreau] peux-tu valider ?',NULL,'10001','3','10000',19,16,19,NULL,'2026-03-14 18:31:29.000','2026-04-05 17:59:37.000','2026-04-05 17:59:37.000',NULL,NULL,NULL,NULL,0,'0|i02osg:',10003);
INSERT INTO `issues` VALUES (10140,10001,38,'10001',NULL,'En tant que client, je veux suivre ma livraison','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* La page répond en moins d\'une seconde\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412].',NULL,'3','4',NULL,18,2,1,NULL,'2026-03-17 15:43:52.000','2026-03-31 09:24:45.000',NULL,'2026-03-26',111600,111600,NULL,0,'0|i01o1s:',NULL);
INSERT INTO `issues` VALUES (10141,10001,39,'10001',NULL,'En tant que agent du support, je veux voir l\'historique de mes demandes',NULL,NULL,'3','2',NULL,9,5,9,NULL,'2026-03-19 10:47:38.000','2026-06-07 14:54:46.000',NULL,'2026-03-26',75600,75600,NULL,0,'0|i00ao0:',NULL);
INSERT INTO `issues` VALUES (10142,10000,65,'10000',NULL,'Application mobile v2','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Le résultat est identique sur mobile et sur ordinateur\n\nLié à DEMO-30. [~hugo.robert] peux-tu valider ?',NULL,'10000','2',NULL,16,7,16,NULL,'2026-03-19 10:52:27.000','2026-04-17 17:51:36.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00b28:',NULL);
INSERT INTO `issues` VALUES (10143,10003,27,'3',NULL,'Mettre à jour PostgreSQL sur srv-web-01','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Le résultat est identique sur mobile et sur ordinateur\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}\n\n[~hugo.robert] peux-tu valider ?',NULL,'6','1','2',9,22,9,NULL,'2026-03-19 13:31:54.000','2026-04-10 06:14:40.000','2026-03-26 11:16:48.000',NULL,NULL,NULL,NULL,0,'0|i01z40:',NULL);
INSERT INTO `issues` VALUES (10144,10003,28,'3',NULL,'Mettre à jour PostgreSQL sur srv-db-01','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Un message clair s\'affiche en cas d\'erreur\n* Les tests de bout en bout couvrent le cas nominal\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}',NULL,'6','4','1',22,11,22,NULL,'2026-03-20 18:09:16.000','2026-04-04 06:25:35.000','2026-03-22 13:16:27.000',NULL,54000,54000,NULL,0,'0|i034zk:',NULL);
INSERT INTO `issues` VALUES (10145,10001,40,'10000',NULL,'Performance des pages produit','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Les droits existants sont respectés\n\n[~lucas.bernard] peux-tu valider ?',NULL,'10000','2',NULL,11,6,11,NULL,'2026-03-23 16:24:39.000','2026-05-03 14:36:16.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00bgg:',NULL);
INSERT INTO `issues` VALUES (10146,10000,66,'1',NULL,'La recherche avancée se vide au rafraîchissement','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\n*Étapes pour reproduire :*\n# Ouvrir le tableau de bord\n# Rafraîchir la page\n# Ajouter deux articles au panier\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.',NULL,'10000','5',NULL,11,13,11,NULL,'2026-03-26 11:43:20.000','2026-03-29 14:15:13.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01urk:',10050);
INSERT INTO `issues` VALUES (10147,10000,67,'5',10091,'Déployer en préproduction','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Un message clair s\'affiche en cas d\'erreur\n\n[~paul.martin] peux-tu valider ?\n\n!maquette.jpg!',NULL,'10001','3','2',10,17,10,NULL,'2026-03-26 16:17:13.000','2026-04-19 08:10:11.000','2026-04-02 08:11:36.000','2026-04-26',NULL,NULL,NULL,0,'0|i02p6o:',NULL);
INSERT INTO `issues` VALUES (10148,10003,29,'1',NULL,'Superviser la file de messages sur srv-web-01','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\n*Étapes pour reproduire :*\n# Ouvrir le tableau de bord\n# Rafraîchir la page\n# Se connecter avec un compte client\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.','Recette — https://recette.exemple.fr','6','3','1',11,11,1,NULL,'2026-03-28 12:38:44.000','2026-04-29 06:50:55.000','2026-04-17 17:11:36.000',NULL,NULL,NULL,NULL,0,'0|i02pkw:',NULL);
INSERT INTO `issues` VALUES (10149,10001,41,'10001',NULL,'En tant que administrateur, je veux partager un panier','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Le résultat est identique sur mobile et sur ordinateur\n* Les tests de bout en bout couvrent le cas nominal',NULL,'10001','3','2',15,7,15,NULL,'2026-04-04 12:27:57.000','2026-04-30 08:56:44.000','2026-04-10 17:49:23.000',NULL,72000,49500,22500,0,'0|i02pz4:',NULL);
INSERT INTO `issues` VALUES (10150,10002,14,'3',NULL,'Mettre à jour la grille salariale',NULL,NULL,'6','3','5',14,10,14,NULL,'2026-04-04 16:27:25.000','2026-04-26 15:02:13.000','2026-04-15 12:38:01.000',NULL,NULL,NULL,39600,0,'0|i02qdc:',NULL);
INSERT INTO `issues` VALUES (10151,10002,15,'5',10082,'Déployer en préproduction',NULL,NULL,'5','3','1',10,14,10,NULL,'2026-04-07 06:11:29.000','2026-05-05 11:11:24.000','2026-04-11 10:04:41.000',NULL,NULL,NULL,NULL,0,'0|i02qrk:',NULL);
INSERT INTO `issues` VALUES (10152,10000,68,'3',NULL,'Tester l\'export CSV des factures','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* La page répond en moins d\'une seconde\n\n[~thomas.moreau] peux-tu valider ?\n\n!maquette.jpg!',NULL,'10000','4',NULL,10,17,10,NULL,'2026-04-08 07:19:54.000','2026-05-01 16:29:05.000',NULL,NULL,64800,64800,NULL,0,'0|i01og0:',10133);
INSERT INTO `issues` VALUES (10153,10001,42,'10001',NULL,'En tant que administrateur, je veux exporter mes factures','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412]. Lié à WEB-36.',NULL,'10001','2','10000',2,15,2,NULL,'2026-04-10 08:17:14.000','2026-05-01 14:02:17.000','2026-04-27 10:20:26.000','2026-05-24',133200,133200,NULL,0,'0|i02680:',NULL);
INSERT INTO `issues` VALUES (10154,10000,69,'10000',NULL,'Application mobile v2',NULL,NULL,'3','3',NULL,20,17,20,NULL,'2026-04-11 08:55:33.000','2026-04-25 06:28:31.000',NULL,NULL,39600,39600,NULL,0,'0|i0114w:',NULL);
INSERT INTO `issues` VALUES (10155,10000,70,'10001',NULL,'En tant que commercial, je veux réinitialiser mon mot de passe',NULL,NULL,'3','2',NULL,10,17,10,2,'2026-04-12 12:13:02.000','2026-04-24 13:07:35.000',NULL,NULL,144000,144000,NULL,0,'0|i00buo:',10105);
INSERT INTO `issues` VALUES (10156,10000,71,'3',NULL,'Automatiser les traductions anglaises','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur\n* Le résultat est identique sur mobile et sur ordinateur\n\nVoir [la maquette|https://maquettes.exemple.fr/panier].',NULL,'10000','3',NULL,15,6,15,NULL,'2026-04-14 07:32:22.000','2026-04-28 17:23:08.000',NULL,'2026-05-13',NULL,NULL,NULL,0,'0|i011j4:',NULL);
INSERT INTO `issues` VALUES (10157,10002,16,'3',NULL,'Préparer la visite médicale de Mme Petit','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les droits existants sont respectés\n* Le résultat est identique sur mobile et sur ordinateur\n\nVoir [la documentation interne|https://wiki.exemple.fr/exploitation/sauvegardes].',NULL,'5','2','1',8,8,8,NULL,'2026-04-14 14:52:40.000','2026-04-22 13:37:03.000','2026-04-22 13:37:03.000',NULL,111600,111600,NULL,0,'0|i026m8:',NULL);
INSERT INTO `issues` VALUES (10158,10000,72,'3',NULL,'Mettre à jour les tests de bout en bout','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}\n\n[~thomas.moreau] peux-tu valider ?',NULL,'10000','3',NULL,17,3,17,NULL,'2026-04-15 12:16:45.000','2026-05-13 14:42:52.000',NULL,NULL,NULL,NULL,15300,0,'0|i011xc:',10133);
INSERT INTO `issues` VALUES (10159,10003,30,'1',NULL,'Redémarrage inopiné de srv-db-02','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\n*Étapes pour reproduire :*\n# Se connecter avec un compte client\n# Changer la langue en anglais\n# Lancer l\'export\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.','Chrome 129 / Windows 11','1','1',NULL,2,7,2,NULL,'2026-04-16 07:40:40.000','2026-04-22 14:11:04.000',NULL,NULL,NULL,NULL,NULL,0,'0|i002rk:',NULL);
INSERT INTO `issues` VALUES (10160,10001,43,'10001',NULL,'En tant que commercial, je veux exporter mes factures','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412].',NULL,'10001','3','2',11,11,11,NULL,'2026-04-16 17:00:09.000','2026-05-05 13:14:39.000','2026-05-05 13:14:39.000',NULL,NULL,NULL,NULL,0,'0|i02r5s:',10088);
INSERT INTO `issues` VALUES (10161,10001,44,'1',NULL,'Le tableau de bord perd les données saisies','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\n*Étapes pour reproduire :*\n# Ouvrir le tableau de bord\n# Changer la langue en anglais\n# Lancer l\'export\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\nLié à WEB-14.',NULL,'10000','1',NULL,11,17,11,NULL,'2026-04-17 11:55:18.000','2026-05-11 11:32:00.000',NULL,NULL,50400,50400,NULL,0,'0|i0035s:',10145);
INSERT INTO `issues` VALUES (10162,10003,31,'4',NULL,'Augmenter l\'espace disque sur srv-web-02','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur\n* Les tests de bout en bout couvrent le cas nominal\n\nVoir [la documentation interne|https://wiki.exemple.fr/exploitation/sauvegardes].',NULL,'5','4','2',9,NULL,9,NULL,'2026-04-17 15:11:28.000','2026-05-13 15:29:51.000','2026-04-29 12:20:02.000','2026-06-08',NULL,NULL,22500,0,'0|i035ds:',NULL);
INSERT INTO `issues` VALUES (10163,10000,73,'10001',NULL,'En tant que administrateur, je veux suivre ma livraison','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}',NULL,'10001','1','10000',2,NULL,2,NULL,'2026-04-18 11:37:05.000','2026-05-02 17:50:36.000','2026-04-24 14:44:41.000',NULL,NULL,NULL,39600,0,'0|i01zi8:',NULL);
INSERT INTO `issues` VALUES (10164,10001,45,'3',NULL,'Refondre le parcours d\'inscription','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal',NULL,'10002','2',NULL,5,2,5,NULL,'2026-04-19 08:56:29.000','2026-05-03 15:48:43.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00c8w:',NULL);
INSERT INTO `issues` VALUES (10165,10001,46,'10001',NULL,'En tant que commercial, je veux filtrer les commandes par statut','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n* Un message clair s\'affiche en cas d\'erreur\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412]. [~ines.david] peux-tu valider ?',NULL,'10001','4','10000',15,NULL,15,NULL,'2026-04-19 13:46:10.000','2026-05-07 10:42:18.000','2026-05-07 10:42:18.000','2026-06-17',108000,108000,NULL,0,'0|i035s0:',10088);
INSERT INTO `issues` VALUES (10166,10003,32,'3',NULL,'Ouvrir le flux réseau entre srv-web-01 et le VPN','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les droits existants sont respectés\n\n[~paul.martin] peux-tu valider ?',NULL,'6','2','4',23,7,23,NULL,'2026-04-21 13:32:48.000','2026-04-28 11:30:53.000','2026-04-28 07:56:56.000',NULL,111600,111600,NULL,0,'0|i0270g:',NULL);
INSERT INTO `issues` VALUES (10167,10001,47,'3',NULL,'Refondre la configuration du cache','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n* Un message clair s\'affiche en cas d\'erreur\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}\n\nVoir [la maquette|https://maquettes.exemple.fr/panier]. Lié à WEB-31.',NULL,'10001','3','10000',2,17,1,NULL,'2026-04-21 14:33:18.000','2026-05-08 08:24:09.000','2026-05-07 11:02:36.000','2026-05-03',90000,90000,NULL,0,'0|i02rk0:',NULL);
INSERT INTO `issues` VALUES (10168,10001,48,'10001',NULL,'En tant que administrateur, je veux partager un panier',NULL,NULL,'3','3',NULL,18,9,18,NULL,'2026-04-21 15:18:10.000','2026-05-08 11:20:57.000',NULL,'2026-05-01',100800,100800,NULL,0,'0|i012bk:',NULL);
INSERT INTO `issues` VALUES (10169,10000,74,'5',10056,'Mettre à jour la documentation','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* La page répond en moins d\'une seconde\n* Le résultat est identique sur mobile et sur ordinateur',NULL,'3','3',NULL,3,13,3,NULL,'2026-04-23 13:55:39.000','2026-05-07 06:42:50.000',NULL,NULL,10800,10800,NULL,0,'0|i012ps:',NULL);
INSERT INTO `issues` VALUES (10170,10001,49,'1',NULL,'Le calendrier des livraisons s\'affiche mal sur mobile','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\n*Étapes pour reproduire :*\n# Changer la langue en anglais\n# Ajouter deux articles au panier\n# Lancer l\'export\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n[~camille.petit] peux-tu valider ?',NULL,'10000','4',NULL,13,NULL,13,NULL,'2026-04-23 14:25:11.000','2026-05-23 08:54:48.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01ou8:',NULL);
INSERT INTO `issues` VALUES (10171,10002,17,'4',NULL,'Préparer l\'entretien annuel de Mme Benali',NULL,NULL,'5','1','1',14,8,14,NULL,'2026-04-24 17:57:33.000','2026-05-12 08:37:13.000','2026-05-04 06:27:09.000',NULL,NULL,NULL,32400,0,'0|i01zwg:',NULL);
INSERT INTO `issues` VALUES (10172,10001,50,'10001',NULL,'En tant que client, je veux suivre ma livraison','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Les tests de bout en bout couvrent le cas nominal\n* Un message clair s\'affiche en cas d\'erreur\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}',NULL,'10000','3',NULL,15,6,15,NULL,'2026-04-27 06:02:45.000','2026-05-13 16:21:02.000',NULL,NULL,108000,108000,NULL,0,'0|i01340:',NULL);
INSERT INTO `issues` VALUES (10173,10000,75,'3',NULL,'Refondre la page d\'accueil','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* La page répond en moins d\'une seconde',NULL,'10000','2',NULL,15,6,15,NULL,'2026-04-28 08:50:28.000','2026-05-07 17:59:44.000',NULL,'2026-06-03',NULL,NULL,35100,0,'0|i00cn4:',10080);
INSERT INTO `issues` VALUES (10174,10000,76,'10001',NULL,'En tant que commercial, je veux exporter mes factures',NULL,NULL,'10002','3',NULL,2,2,2,NULL,'2026-04-29 08:46:05.000','2026-05-24 10:37:08.000',NULL,NULL,NULL,NULL,44100,0,'0|i013i8:',NULL);
INSERT INTO `issues` VALUES (10175,10003,33,'3',NULL,'Ouvrir le flux réseau entre srv-batch-01 et le VPN','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Les tests de bout en bout couvrent le cas nominal\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}\n\nVoir [la documentation interne|https://wiki.exemple.fr/exploitation/sauvegardes]. Lié à OPS-27.',NULL,'3','3',NULL,21,7,21,NULL,'2026-05-02 16:30:59.000','2026-06-25 08:03:02.000',NULL,NULL,NULL,NULL,NULL,0,'0|i013wg:',NULL);
INSERT INTO `issues` VALUES (10176,10000,77,'3',NULL,'Documenter la page d\'accueil','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Les droits existants sont respectés\n\nLié à DEMO-28. [~jeanne.dupont] peux-tu valider ?',NULL,'10000','3',NULL,12,4,1,NULL,'2026-05-04 12:10:37.000','2026-06-03 15:17:25.000',NULL,'2026-05-20',NULL,NULL,NULL,0,'0|i014ao:',NULL);
INSERT INTO `issues` VALUES (10177,10003,34,'3',NULL,'Mettre à jour PostgreSQL sur proxy-01',NULL,NULL,'1','3',NULL,12,7,12,NULL,'2026-05-04 16:39:04.000','2026-05-17 09:14:51.000',NULL,NULL,28800,28800,NULL,0,'0|i014ow:',NULL);
INSERT INTO `issues` VALUES (10178,10001,51,'1',NULL,'Le formulaire d\'inscription ne respecte pas les droits d\'accès','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\n*Étapes pour reproduire :*\n# Lancer l\'export\n# Se connecter avec un compte client\n# Ajouter deux articles au panier\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\nVoir [la maquette|https://maquettes.exemple.fr/panier]. Lié à WEB-1.',NULL,'10001','3','10000',11,2,11,NULL,'2026-05-10 15:34:55.000','2026-06-07 07:10:50.000','2026-05-23 12:32:45.000',NULL,57600,6300,51300,0,'0|i02ry8:',10145);
INSERT INTO `issues` VALUES (10179,10000,78,'3',NULL,'Simplifier l\'export CSV des factures','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur',NULL,'10000','4',NULL,4,2,4,NULL,'2026-05-12 06:23:26.000','2026-05-22 07:52:20.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01p8g:',10105);
INSERT INTO `issues` VALUES (10180,10002,18,'4',NULL,'Mettre à jour la grille salariale','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur',NULL,'4','4',NULL,14,NULL,14,NULL,'2026-05-12 07:43:46.000','2026-05-29 13:45:24.000',NULL,NULL,136800,136800,NULL,0,'0|i01pmo:',NULL);
INSERT INTO `issues` VALUES (10181,10000,79,'3',NULL,'Tester la page de profil','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Un message clair s\'affiche en cas d\'erreur\n* Les droits existants sont respectés',NULL,'3','2',NULL,20,16,20,2,'2026-05-12 15:59:29.000','2026-06-08 09:10:13.000',NULL,NULL,86400,86400,NULL,0,'0|i00d1c:',10050);
INSERT INTO `issues` VALUES (10182,10001,52,'10001',NULL,'En tant que commercial, je veux suivre ma livraison',NULL,NULL,'10000','2',NULL,7,7,7,NULL,'2026-05-13 06:11:15.000','2026-05-21 16:06:11.000',NULL,NULL,64800,64800,NULL,0,'0|i00dfk:',NULL);
INSERT INTO `issues` VALUES (10183,10001,53,'10000',NULL,'Application mobile v2','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur\n\n!capture.png!',NULL,'3','2',NULL,4,7,1,NULL,'2026-05-13 14:25:54.000','2026-06-01 07:47:46.000',NULL,NULL,25200,25200,NULL,0,'0|i00dts:',NULL);
INSERT INTO `issues` VALUES (10184,10000,80,'1',NULL,'Le formulaire d\'inscription perd les données saisies','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\n*Étapes pour reproduire :*\n# Se connecter avec un compte client\n# Ajouter deux articles au panier\n# Rafraîchir la page\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}\n\n[~lucas.bernard] peux-tu valider ?','Safari 18 / iOS 18','10000','3',NULL,16,7,16,NULL,'2026-05-13 15:02:17.000','2026-09-15 11:00:59.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01534:',NULL);
INSERT INTO `issues` VALUES (10185,10001,54,'5',10052,'Corriger les retours de recette','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* La page répond en moins d\'une seconde',NULL,'10001','3','10000',18,5,1,NULL,'2026-05-14 13:09:42.000','2026-06-06 09:19:53.000','2026-05-24 11:09:03.000',NULL,32400,32400,NULL,0,'0|i02scg:',NULL);
INSERT INTO `issues` VALUES (10186,10000,81,'1',NULL,'Les notifications e-mail ne respecte pas les droits d\'accès','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\n*Étapes pour reproduire :*\n# Rafraîchir la page\n# Lancer l\'export\n# Changer la langue en anglais\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\nLié à DEMO-55.','Safari 18 / iOS 18','10001','4','10000',13,15,13,NULL,'2026-05-15 17:48:30.000','2026-05-30 07:20:32.000','2026-05-30 07:20:32.000',NULL,NULL,NULL,NULL,0,'0|i03668:',10003);
INSERT INTO `issues` VALUES (10187,10003,35,'3',NULL,'Superviser la file de messages sur proxy-01','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Un message clair s\'affiche en cas d\'erreur\n* Le résultat est identique sur mobile et sur ordinateur\n\n[~marie.durand] peux-tu valider ?',NULL,'1','2',NULL,19,22,19,NULL,'2026-05-16 12:03:33.000','2026-06-04 11:56:00.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00e80:',NULL);
INSERT INTO `issues` VALUES (10188,10000,82,'5',10041,'Corriger les retours de recette','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Un message clair s\'affiche en cas d\'erreur\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}\n\nLié à DEMO-74. [~marie.durand] peux-tu valider ?',NULL,'10001','3','10000',3,NULL,3,2,'2026-05-17 09:56:14.000','2026-06-11 13:45:06.000','2026-06-11 13:45:06.000','2026-05-27',NULL,NULL,2700,0,'0|i02sqo:',NULL);
INSERT INTO `issues` VALUES (10189,10000,83,'5',10032,'Préparer la recette',NULL,NULL,'10001','3','10000',2,3,2,NULL,'2026-05-17 10:31:34.000','2026-06-10 11:41:22.000','2026-06-10 11:41:22.000',NULL,NULL,NULL,NULL,0,'0|i02t4w:',NULL);
INSERT INTO `issues` VALUES (10190,10000,84,'10001',NULL,'En tant que administrateur, je veux voir l\'historique de mes demandes','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Les droits existants sont respectés\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}\n\nVoir [la documentation interne|https://wiki.exemple.fr/exploitation/sauvegardes].',NULL,'3','5',NULL,5,5,5,NULL,'2026-05-18 09:33:35.000','2026-06-26 13:11:45.000',NULL,NULL,100800,100800,NULL,0,'0|i01v5s:',NULL);
INSERT INTO `issues` VALUES (10191,10003,36,'3',NULL,'Redémarrage inopiné de srv-web-02','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Un message clair s\'affiche en cas d\'erreur',NULL,'1','2',NULL,22,21,22,NULL,'2026-05-18 12:03:10.000','2026-05-24 09:05:39.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00em8:',NULL);
INSERT INTO `issues` VALUES (10192,10000,85,'5',10074,'Déployer en préproduction','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Les tests de bout en bout couvrent le cas nominal\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}',NULL,'10000','3',NULL,7,16,7,NULL,'2026-05-19 08:17:45.000','2026-06-14 12:02:09.000',NULL,NULL,NULL,NULL,NULL,0,'0|i015hc:',NULL);
INSERT INTO `issues` VALUES (10193,10000,86,'10001',NULL,'En tant que agent du support, je veux recevoir une alerte de stock','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Le résultat est identique sur mobile et sur ordinateur\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}\n\n[~ines.david] peux-tu valider ?',NULL,'10000','3',NULL,10,17,1,NULL,'2026-05-20 11:38:32.000','2026-05-21 16:34:22.000',NULL,NULL,NULL,NULL,NULL,0,'0|i015vk:',10133);
INSERT INTO `issues` VALUES (10194,10002,19,'3',NULL,'Valider les congés d\'été','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Les droits existants sont respectés\n* La page répond en moins d\'une seconde\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}',NULL,'1','3',NULL,8,8,8,NULL,'2026-05-20 15:41:35.000','2026-06-18 10:13:29.000',NULL,NULL,NULL,NULL,NULL,0,'0|i0169s:',NULL);
INSERT INTO `issues` VALUES (10195,10000,87,'10001',NULL,'En tant que commercial, je veux recevoir une alerte de stock','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les droits existants sont respectés\n* Les tests de bout en bout couvrent le cas nominal',NULL,'10002','2',NULL,10,NULL,1,NULL,'2026-05-21 17:39:39.000','2026-06-20 06:30:22.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00f0g:',10142);
INSERT INTO `issues` VALUES (10196,10001,55,'10000',NULL,'Nouveau back-office','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Le résultat est identique sur mobile et sur ordinateur\n* Un message clair s\'affiche en cas d\'erreur',NULL,'10001','2','10000',19,6,19,NULL,'2026-05-22 15:15:22.000','2026-06-14 16:32:03.000','2026-06-14 16:32:03.000',NULL,46800,46800,NULL,0,'0|i027eo:',NULL);
INSERT INTO `issues` VALUES (10197,10002,20,'5',10028,'Revue de code','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}',NULL,'1','3',NULL,8,14,8,NULL,'2026-05-24 06:33:07.000','2026-06-09 06:53:47.000',NULL,NULL,NULL,NULL,NULL,0,'0|i016o0:',NULL);
INSERT INTO `issues` VALUES (10198,10001,56,'10001',NULL,'En tant que agent du support, je veux filtrer les commandes par statut','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Le résultat est identique sur mobile et sur ordinateur\n* Les tests de bout en bout couvrent le cas nominal\n\nVoir [la documentation interne|https://wiki.exemple.fr/exploitation/sauvegardes]. Lié à WEB-39.',NULL,'3','3',NULL,19,3,19,NULL,'2026-05-24 11:46:04.000','2026-06-08 15:41:05.000',NULL,NULL,NULL,NULL,12600,0,'0|i01728:',10183);
INSERT INTO `issues` VALUES (10199,10000,88,'10001',NULL,'En tant que administrateur, je veux réinitialiser mon mot de passe','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Le résultat est identique sur mobile et sur ordinateur\n* Les tests de bout en bout couvrent le cas nominal\n\n[~lucas.bernard] peux-tu valider ?',NULL,'3','3',NULL,3,11,3,NULL,'2026-05-25 06:04:02.000','2026-06-21 16:27:34.000',NULL,NULL,NULL,NULL,NULL,0,'0|i017gg:',NULL);
INSERT INTO `issues` VALUES (10200,10000,89,'10001',NULL,'En tant que agent du support, je veux exporter mes factures','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Le résultat est identique sur mobile et sur ordinateur\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}',NULL,'10001','2','10000',4,2,4,NULL,'2026-05-25 09:07:30.000','2026-06-16 08:13:15.000','2026-06-16 08:13:15.000',NULL,144000,144000,NULL,0,'0|i027sw:',10080);
INSERT INTO `issues` VALUES (10201,10000,90,'10001',NULL,'En tant que client, je veux réinitialiser mon mot de passe','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Les droits existants sont respectés\n* Les tests de bout en bout couvrent le cas nominal\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}',NULL,'10000','2',NULL,7,9,7,NULL,'2026-05-26 09:53:49.000','2026-06-17 10:29:04.000',NULL,NULL,NULL,NULL,20700,0,'0|i00feo:',NULL);
INSERT INTO `issues` VALUES (10202,10001,57,'1',NULL,'L\'import CSV perd les données saisies','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\n*Étapes pour reproduire :*\n# Se connecter avec un compte client\n# Changer la langue en anglais\n# Ouvrir le tableau de bord\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.','Safari 18 / iOS 18','10001','1','10000',4,5,4,NULL,'2026-05-26 12:08:09.000','2026-06-14 10:24:39.000','2026-06-05 09:53:17.000',NULL,72000,72000,NULL,0,'0|i020ao:',10183);
INSERT INTO `issues` VALUES (10203,10002,21,'3',NULL,'Renouveler la mutuelle d\'entreprise','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}',NULL,'5','2','4',14,10,14,NULL,'2026-05-27 10:26:03.000','2026-06-05 08:41:27.000','2026-06-05 08:41:27.000',NULL,NULL,NULL,NULL,0,'0|i02874:',NULL);
INSERT INTO `issues` VALUES (10204,10003,37,'4',NULL,'Renouveler le certificat TLS de proxy-01','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal',NULL,'6','3','2',12,11,12,NULL,'2026-05-28 09:10:26.000','2026-07-08 11:27:20.000','2026-05-29 15:18:52.000',NULL,NULL,NULL,32400,0,'0|i02tj4:',NULL);
INSERT INTO `issues` VALUES (10205,10003,38,'4',NULL,'Sauvegarde nocturne en échec sur srv-web-02','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur',NULL,'1','1',NULL,23,NULL,23,NULL,'2026-05-29 12:02:21.000','2026-06-17 09:54:20.000',NULL,NULL,61200,61200,NULL,0,'0|i003k0:',NULL);
INSERT INTO `issues` VALUES (10206,10000,91,'10001',NULL,'En tant que agent du support, je veux exporter mes factures','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Un message clair s\'affiche en cas d\'erreur\n* Le résultat est identique sur mobile et sur ordinateur',NULL,'3','3',NULL,12,NULL,12,NULL,'2026-05-30 06:03:02.000','2026-06-01 13:43:52.000',NULL,NULL,54000,54000,NULL,0,'0|i017uo:',NULL);
INSERT INTO `issues` VALUES (10207,10001,58,'3',NULL,'Documenter la page de profil','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Le résultat est identique sur mobile et sur ordinateur\n* Les droits existants sont respectés\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}\n\nVoir [la maquette|https://maquettes.exemple.fr/panier]. [~camille.petit] peux-tu valider ?',NULL,'10002','3',NULL,16,17,16,NULL,'2026-05-30 10:00:06.000','2026-07-04 07:24:28.000',NULL,NULL,NULL,NULL,NULL,0,'0|i0188w:',NULL);
INSERT INTO `issues` VALUES (10208,10000,92,'10001',NULL,'En tant que administrateur, je veux réinitialiser mon mot de passe','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Un message clair s\'affiche en cas d\'erreur\n* Les droits existants sont respectés\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}',NULL,'10001','3','10000',10,17,10,NULL,'2026-06-01 07:41:38.000','2026-06-19 15:23:12.000','2026-06-12 13:51:15.000','2026-06-19',NULL,NULL,9900,0,'0|i02txc:',NULL);
INSERT INTO `issues` VALUES (10209,10003,39,'5',10065,'Mettre à jour la documentation','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur\n\nLié à OPS-31.',NULL,'1','2',NULL,21,21,21,NULL,'2026-06-02 17:06:34.000','2026-06-09 12:35:51.000',NULL,'2026-07-23',NULL,NULL,NULL,0,'0|i00fsw:',NULL);
INSERT INTO `issues` VALUES (10210,10001,59,'10001',NULL,'En tant que commercial, je veux réinitialiser mon mot de passe','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur\n* La page répond en moins d\'une seconde\n\n{code:sql}\nSELECT id, statut FROM commande\nWHERE cree_le > now() - interval \'7 days\';\n{code}',NULL,'3','2',NULL,17,17,17,NULL,'2026-06-06 09:51:14.000','2026-06-17 17:10:40.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00g74:',NULL);
INSERT INTO `issues` VALUES (10211,10000,93,'1',NULL,'Le formulaire d\'inscription plante sur Safari 18','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\n*Étapes pour reproduire :*\n# Lancer l\'export\n# Se connecter avec un compte client\n# Ajouter deux articles au panier\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412]. Lié à DEMO-50.','Recette — https://recette.exemple.fr','3','2',NULL,2,11,2,NULL,'2026-06-06 16:16:50.000','2026-07-06 11:15:28.000',NULL,NULL,115200,115200,NULL,0,'0|i00glc:',NULL);
INSERT INTO `issues` VALUES (10212,10000,94,'10001',NULL,'En tant que administrateur, je veux exporter mes factures','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* La page répond en moins d\'une seconde\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}\n\nLié à DEMO-83.',NULL,'10001','3','10000',2,NULL,2,NULL,'2026-06-07 06:13:42.000','2026-06-28 06:58:31.000','2026-06-28 06:58:31.000',NULL,NULL,NULL,NULL,0,'0|i02ubk:',NULL);
INSERT INTO `issues` VALUES (10213,10003,40,'1',NULL,'Augmenter l\'espace disque sur proxy-01','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\n*Étapes pour reproduire :*\n# Se connecter avec un compte client\n# Ajouter deux articles au panier\n# Changer la langue en anglais\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.',NULL,'1','2',NULL,7,11,7,NULL,'2026-06-10 12:00:11.000','2026-06-25 17:39:49.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00gzk:',NULL);
INSERT INTO `issues` VALUES (10214,10001,60,'5',10111,'Revue de code','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* La page répond en moins d\'une seconde\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412].',NULL,'10000','4',NULL,16,2,16,NULL,'2026-06-10 13:24:01.000','2026-06-30 10:56:12.000',NULL,'2026-06-30',NULL,NULL,50400,0,'0|i01q0w:',NULL);
INSERT INTO `issues` VALUES (10215,10001,61,'10001',NULL,'En tant que client, je veux suivre ma livraison','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* La page répond en moins d\'une seconde\n* Le résultat est identique sur mobile et sur ordinateur\n\nVoir [la documentation interne|https://wiki.exemple.fr/exploitation/sauvegardes].',NULL,'3','4',NULL,19,15,19,NULL,'2026-06-12 15:08:32.000','2026-07-07 06:23:08.000',NULL,'2026-07-22',72000,72000,NULL,0,'0|i01qf4:',NULL);
INSERT INTO `issues` VALUES (10216,10001,62,'5',10039,'Déployer en préproduction','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Les droits existants sont respectés\n* Les tests de bout en bout couvrent le cas nominal',NULL,'3','2',NULL,12,13,12,NULL,'2026-06-14 08:34:23.000','2026-07-24 11:59:37.000',NULL,NULL,43200,43200,NULL,0,'0|i00hds:',NULL);
INSERT INTO `issues` VALUES (10217,10000,95,'3',NULL,'Automatiser la page d\'accueil','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Un message clair s\'affiche en cas d\'erreur\n* Les tests de bout en bout couvrent le cas nominal\n\n[~paul.martin] peux-tu valider ?',NULL,'3','3',NULL,6,17,6,NULL,'2026-06-14 12:08:12.000','2026-07-21 11:27:04.000',NULL,NULL,NULL,NULL,45000,0,'0|i018n4:',10105);
INSERT INTO `issues` VALUES (10218,10001,63,'10001',NULL,'En tant que agent du support, je veux exporter mes factures','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Un message clair s\'affiche en cas d\'erreur\n\n[~nicolas.laurent] peux-tu valider ?',NULL,'3','3',NULL,7,6,7,NULL,'2026-06-15 08:41:40.000','2026-06-30 15:30:52.000',NULL,NULL,NULL,NULL,NULL,0,'0|i0191c:',10145);
INSERT INTO `issues` VALUES (10219,10000,96,'10001',NULL,'En tant que agent du support, je veux voir l\'historique de mes demandes',NULL,NULL,'10002','3',NULL,17,15,17,NULL,'2026-06-16 10:09:19.000','2026-07-06 09:32:24.000',NULL,NULL,NULL,NULL,NULL,0,'0|i019fk:',10154);
INSERT INTO `issues` VALUES (10220,10003,41,'3',NULL,'Sauvegarde nocturne en échec sur srv-web-02','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal',NULL,'6','2','1',11,7,11,NULL,'2026-06-16 17:27:45.000','2026-07-05 16:47:57.000','2026-06-22 14:05:26.000',NULL,NULL,NULL,NULL,0,'0|i028lc:',NULL);
INSERT INTO `issues` VALUES (10221,10001,64,'1',NULL,'Le panier s\'affiche mal sur mobile','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\n*Étapes pour reproduire :*\n# Rafraîchir la page\n# Changer la langue en anglais\n# Se connecter avec un compte client\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\nLié à WEB-29.','Firefox 131 / macOS 15','10002','1',NULL,2,3,2,NULL,'2026-06-17 11:34:33.000','2026-07-04 15:44:52.000',NULL,'2026-07-17',61200,61200,NULL,0,'0|i003y8:',NULL);
INSERT INTO `issues` VALUES (10222,10001,65,'10001',NULL,'En tant que client, je veux filtrer les commandes par statut','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n\nLié à WEB-30.',NULL,'10000','4',NULL,17,4,17,NULL,'2026-06-18 11:27:29.000','2026-07-13 14:56:39.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01qtc:',NULL);
INSERT INTO `issues` VALUES (10223,10001,66,'5',10178,'Mettre à jour la documentation','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur',NULL,'10001','3','2',7,2,7,NULL,'2026-06-19 16:47:38.000','2026-07-27 08:03:34.000','2026-07-04 07:03:03.000',NULL,NULL,NULL,NULL,0,'0|i02ups:',NULL);
INSERT INTO `issues` VALUES (10224,10003,42,'3',NULL,'Ouvrir le flux réseau entre srv-web-02 et le VPN','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Le résultat est identique sur mobile et sur ordinateur\n* Les droits existants sont respectés\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}',NULL,'6','2','1',9,23,9,NULL,'2026-06-20 06:53:07.000','2026-07-10 15:36:15.000','2026-06-26 14:21:08.000',NULL,NULL,NULL,NULL,0,'0|i028zk:',NULL);
INSERT INTO `issues` VALUES (10225,10002,22,'3',NULL,'Préparer l\'entretien annuel de Mme Lambert','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur\n\n[~nicolas.laurent] peux-tu valider ?',NULL,'6','2','1',14,8,14,NULL,'2026-06-20 11:09:37.000','2026-07-19 15:52:59.000','2026-07-10 10:10:24.000',NULL,36000,19800,16200,0,'0|i029ds:',NULL);
INSERT INTO `issues` VALUES (10226,10003,43,'1',NULL,'Augmenter l\'espace disque sur proxy-01','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\n*Étapes pour reproduire :*\n# Rafraîchir la page\n# Se connecter avec un compte client\n# Changer la langue en anglais\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412].',NULL,'6','3','1',12,NULL,12,NULL,'2026-06-21 12:31:49.000','2026-07-10 09:59:37.000','2026-07-01 12:42:46.000','2026-08-03',43200,43200,NULL,0,'0|i02v40:',NULL);
INSERT INTO `issues` VALUES (10227,10003,44,'3',NULL,'Sauvegarde nocturne en échec sur srv-db-02','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Les droits existants sont respectés\n* La page répond en moins d\'une seconde\n\nLié à OPS-7.\n\n!capture.png!',NULL,'6','2','1',21,7,21,NULL,'2026-06-21 16:37:58.000','2026-07-20 16:17:52.000','2026-06-30 13:16:00.000',NULL,NULL,NULL,NULL,0,'0|i029s0:',NULL);
INSERT INTO `issues` VALUES (10228,10001,67,'3',NULL,'Migrer la configuration du cache',NULL,NULL,'10000','1',NULL,12,9,12,NULL,'2026-06-22 08:52:05.000','2026-07-14 08:16:13.000',NULL,NULL,NULL,NULL,NULL,0,'0|i004cg:',NULL);
INSERT INTO `issues` VALUES (10229,10000,97,'3',NULL,'Refondre la page de profil','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* La page répond en moins d\'une seconde\n\n[~paul.martin] peux-tu valider ?',NULL,'3','3',NULL,12,5,12,NULL,'2026-07-01 10:12:02.000','2026-07-18 15:29:34.000',NULL,NULL,NULL,NULL,NULL,0,'0|i019ts:',NULL);
INSERT INTO `issues` VALUES (10230,10001,68,'10001',NULL,'En tant que agent du support, je veux réinitialiser mon mot de passe','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Un message clair s\'affiche en cas d\'erreur\n* Les droits existants sont respectés\n\nVoir [la maquette|https://maquettes.exemple.fr/panier].',NULL,'10002','1',NULL,11,17,11,NULL,'2026-07-02 08:15:52.000','2026-07-18 12:25:10.000',NULL,'2026-07-14',46800,46800,NULL,0,'0|i004qo:',10088);
INSERT INTO `issues` VALUES (10231,10001,69,'10001',NULL,'En tant que commercial, je veux suivre ma livraison','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur',NULL,'10000','4',NULL,19,7,19,NULL,'2026-07-02 13:15:03.000','2026-08-31 10:52:36.000',NULL,NULL,21600,21600,NULL,0,'0|i01r7k:',10145);
INSERT INTO `issues` VALUES (10232,10001,70,'10001',NULL,'En tant que administrateur, je veux suivre ma livraison','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Un message clair s\'affiche en cas d\'erreur\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}\n\n[~julien.roux] peux-tu valider ?',NULL,'10000','3',NULL,3,3,3,NULL,'2026-07-05 10:15:20.000','2026-07-31 16:07:22.000',NULL,'2026-08-04',NULL,NULL,33300,0,'0|i01a80:',10196);
INSERT INTO `issues` VALUES (10233,10003,45,'1',NULL,'Redémarrage inopiné de srv-db-01','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\n*Étapes pour reproduire :*\n# Ajouter deux articles au panier\n# Lancer l\'export\n# Se connecter avec un compte client\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.','Safari 18 / iOS 18','6','2','1',12,21,12,NULL,'2026-07-05 11:40:30.000','2026-07-22 16:47:04.000','2026-07-18 14:08:23.000',NULL,NULL,NULL,NULL,0,'0|i02a68:',NULL);
INSERT INTO `issues` VALUES (10234,10002,23,'5',10097,'Préparer la recette','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur',NULL,'1','5',NULL,10,10,10,NULL,'2026-07-05 16:43:15.000','2026-08-10 13:00:03.000',NULL,'2026-07-26',NULL,NULL,49500,0,'0|i01vk0:',NULL);
INSERT INTO `issues` VALUES (10235,10000,98,'10001',NULL,'En tant que commercial, je veux réinitialiser mon mot de passe','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Le résultat est identique sur mobile et sur ordinateur',NULL,'10000','2',NULL,9,NULL,9,NULL,'2026-07-07 13:14:58.000','2026-07-21 14:44:36.000',NULL,NULL,25200,25200,NULL,0,'0|i00hs0:',10142);
INSERT INTO `issues` VALUES (10236,10001,71,'1',NULL,'La page de connexion se vide au rafraîchissement','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\n*Étapes pour reproduire :*\n# Changer la langue en anglais\n# Se connecter avec un compte client\n# Rafraîchir la page\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.',NULL,'10001','3','10000',15,17,15,NULL,'2026-07-09 12:25:30.000','2026-07-22 12:28:56.000','2026-07-22 12:28:56.000',NULL,108000,108000,NULL,0,'0|i02vi8:',10145);
INSERT INTO `issues` VALUES (10237,10001,72,'3',NULL,'Migrer l\'export CSV des factures','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur',NULL,'3','3',NULL,2,3,1,NULL,'2026-07-11 08:20:10.000','2026-07-16 11:26:40.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01am8:',NULL);
INSERT INTO `issues` VALUES (10238,10000,99,'3',NULL,'Automatiser le tableau des commandes','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Les droits existants sont respectés',NULL,'10001','1','10000',7,NULL,7,NULL,'2026-07-11 08:55:12.000','2026-08-04 13:38:48.000','2026-08-04 13:38:48.000',NULL,NULL,NULL,NULL,0,'0|i020ow:',10142);
INSERT INTO `issues` VALUES (10239,10000,100,'1',NULL,'L\'import CSV se vide au rafraîchissement',NULL,NULL,'10000','3',NULL,10,16,10,NULL,'2026-07-12 07:09:10.000','2026-08-02 13:55:03.000',NULL,NULL,144000,92700,51300,0,'0|i01b0g:',10003);
INSERT INTO `issues` VALUES (10240,10000,101,'10001',NULL,'En tant que agent du support, je veux recevoir une alerte de stock','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Les tests de bout en bout couvrent le cas nominal\n* Un message clair s\'affiche en cas d\'erreur\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}\n\nLié à DEMO-31.',NULL,'10001','3','10000',18,4,18,NULL,'2026-07-13 07:57:22.000','2026-07-14 17:01:16.000','2026-07-14 17:01:16.000',NULL,NULL,NULL,NULL,0,'0|i02vwg:',NULL);
INSERT INTO `issues` VALUES (10241,10000,102,'10001',NULL,'En tant que administrateur, je veux réinitialiser mon mot de passe','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur\n\nLié à DEMO-29.',NULL,'10001','3','10000',19,5,19,1,'2026-07-14 07:17:28.000','2026-08-10 09:18:35.000','2026-08-10 09:18:35.000',NULL,NULL,NULL,24300,0,'0|i02wao:',NULL);
INSERT INTO `issues` VALUES (10242,10001,73,'10001',NULL,'En tant que agent du support, je veux exporter mes factures','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Les tests de bout en bout couvrent le cas nominal\n\nLié à WEB-17.',NULL,'10000','1',NULL,6,6,6,NULL,'2026-07-14 14:12:31.000','2026-07-19 07:37:47.000',NULL,NULL,NULL,NULL,NULL,0,'0|i0054w:',NULL);
INSERT INTO `issues` VALUES (10243,10002,24,'5',10115,'Corriger les retours de recette',NULL,NULL,'1','1',NULL,8,10,8,NULL,'2026-07-16 09:43:47.000','2026-07-22 08:14:39.000',NULL,'2026-09-12',NULL,NULL,NULL,0,'0|i005j4:',NULL);
INSERT INTO `issues` VALUES (10244,10000,103,'5',10030,'Préparer la recette','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n* Les droits existants sont respectés\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}',NULL,'10000','2',NULL,13,7,13,NULL,'2026-07-16 14:01:31.000','2026-08-29 11:33:15.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00i68:',NULL);
INSERT INTO `issues` VALUES (10245,10000,104,'10001',NULL,'En tant que client, je veux voir l\'historique de mes demandes','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Le résultat est identique sur mobile et sur ordinateur\n\n{code:sql}\nSELECT id, statut FROM commande\nWHERE cree_le > now() - interval \'7 days\';\n{code}\n\nLié à DEMO-65.',NULL,'3','3',NULL,10,2,10,1,'2026-07-18 15:58:26.000','2026-08-28 11:19:44.000',NULL,'2026-08-09',7200,7200,NULL,0,'0|i01beo:',NULL);
INSERT INTO `issues` VALUES (10246,10001,74,'10001',NULL,'En tant que administrateur, je veux exporter mes factures','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n* Un message clair s\'affiche en cas d\'erreur\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412].',NULL,'10000','4',NULL,2,15,2,NULL,'2026-07-19 10:48:08.000','2026-07-24 11:13:53.000',NULL,NULL,64800,64800,NULL,0,'0|i01rls:',10183);
INSERT INTO `issues` VALUES (10247,10002,25,'4',NULL,'Préparer l\'entretien annuel de Mme Benali','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Les droits existants sont respectés\n* Les tests de bout en bout couvrent le cas nominal\n\n{code:sql}\nSELECT id, statut FROM commande\nWHERE cree_le > now() - interval \'7 days\';\n{code}\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}\n\nLié à RH-6.',NULL,'4','3',NULL,1,14,1,NULL,'2026-07-19 15:26:22.000','2026-08-18 09:59:33.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01bsw:',NULL);
INSERT INTO `issues` VALUES (10248,10003,46,'5',10116,'Corriger les retours de recette','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}\n\n!capture.png!',NULL,'1','5',NULL,23,21,23,NULL,'2026-07-23 17:58:33.000','2026-08-12 14:41:26.000',NULL,'2026-08-02',NULL,NULL,19800,0,'0|i01vy8:',NULL);
INSERT INTO `issues` VALUES (10249,10000,105,'10001',NULL,'En tant que administrateur, je veux recevoir une alerte de stock','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* La page répond en moins d\'une seconde\n* Le résultat est identique sur mobile et sur ordinateur\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}',NULL,'10001','3','10000',13,2,1,NULL,'2026-07-25 10:56:50.000','2026-08-18 14:00:09.000','2026-08-18 14:00:09.000','2026-08-22',NULL,NULL,31500,0,'0|i02wow:',NULL);
INSERT INTO `issues` VALUES (10250,10003,47,'3',NULL,'Augmenter l\'espace disque sur srv-db-02','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n* Les droits existants sont respectés',NULL,'3','3',NULL,19,23,19,NULL,'2026-07-26 13:54:39.000','2026-08-05 17:25:15.000',NULL,NULL,133200,94500,38700,0,'0|i01c74:',NULL);
INSERT INTO `issues` VALUES (10251,10000,106,'10001',NULL,'En tant que administrateur, je veux réinitialiser mon mot de passe','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}',NULL,'10000','4',NULL,20,2,20,NULL,'2026-07-27 09:21:12.000','2026-08-14 08:55:39.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01s00:',NULL);
INSERT INTO `issues` VALUES (10252,10001,75,'1',NULL,'La page de connexion se vide au rafraîchissement','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\n*Étapes pour reproduire :*\n# Rafraîchir la page\n# Se connecter avec un compte client\n# Lancer l\'export\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}\n\n[~thomas.moreau] peux-tu valider ?','Production — srv-web-01','10001','4','10000',13,NULL,13,NULL,'2026-07-30 16:40:51.000','2026-08-22 06:24:51.000','2026-08-22 06:24:51.000',NULL,46800,46800,NULL,0,'0|i036kg:',10088);
INSERT INTO `issues` VALUES (10253,10001,76,'1',NULL,'La recherche avancée affiche une date décalée d\'une heure','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\n*Étapes pour reproduire :*\n# Rafraîchir la page\n# Ouvrir le tableau de bord\n# Lancer l\'export\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n[~maxime.bertrand] peux-tu valider ?','Production — srv-web-01','10001','3','2',20,11,20,NULL,'2026-07-31 16:50:46.000','2026-08-16 08:34:24.000','2026-08-11 07:11:34.000',NULL,79200,79200,NULL,0,'0|i02x34:',10145);
INSERT INTO `issues` VALUES (10254,10000,107,'1',NULL,'Les notifications e-mail affiche une date décalée d\'une heure','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\n*Étapes pour reproduire :*\n# Rafraîchir la page\n# Changer la langue en anglais\n# Se connecter avec un compte client\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.',NULL,'10000','4',NULL,12,16,1,NULL,'2026-08-01 07:06:47.000','2026-08-25 17:52:03.000',NULL,NULL,NULL,NULL,55800,0,'0|i01se8:',NULL);
INSERT INTO `issues` VALUES (10255,10002,26,'3',NULL,'Préparer la visite médicale de Mme Benali','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* La page répond en moins d\'une seconde\n\nLié à RH-21.',NULL,'5','3','2',8,14,8,NULL,'2026-08-03 11:46:29.000','2026-08-23 16:37:27.000','2026-08-16 17:32:32.000',NULL,64800,64800,NULL,0,'0|i02xhc:',NULL);
INSERT INTO `issues` VALUES (10256,10001,77,'3',NULL,'Simplifier les traductions anglaises','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les droits existants sont respectés',NULL,'10001','3','10000',15,5,15,NULL,'2026-08-04 16:33:48.000','2026-08-26 06:22:39.000','2026-08-16 11:38:49.000',NULL,NULL,NULL,NULL,0,'0|i02xvk:',NULL);
INSERT INTO `issues` VALUES (10257,10001,78,'1',NULL,'L\'export PDF des factures affiche une date décalée d\'une heure','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\n*Étapes pour reproduire :*\n# Lancer l\'export\n# Se connecter avec un compte client\n# Rafraîchir la page\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}\n\nLié à WEB-13.','Safari 18 / iOS 18','3','2',NULL,10,13,10,NULL,'2026-08-06 07:41:03.000','2026-08-17 11:19:57.000',NULL,NULL,NULL,NULL,17100,0,'0|i00ikg:',NULL);
INSERT INTO `issues` VALUES (10258,10003,48,'1',NULL,'Superviser la file de messages sur srv-web-02','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\n*Étapes pour reproduire :*\n# Lancer l\'export\n# Rafraîchir la page\n# Ajouter deux articles au panier\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}\n\n[~maxime.bertrand] peux-tu valider ?','Firefox 131 / macOS 15','5','2','1',9,23,9,NULL,'2026-08-06 08:16:16.000','2026-08-26 09:42:05.000','2026-08-13 07:50:13.000',NULL,NULL,NULL,NULL,0,'0|i02akg:',NULL);
INSERT INTO `issues` VALUES (10259,10003,49,'3',NULL,'Renouveler le certificat TLS de srv-batch-01','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Les droits existants sont respectés\n\n[~julien.roux] peux-tu valider ?',NULL,'3','2',NULL,2,11,2,NULL,'2026-08-07 16:19:53.000','2026-08-30 07:26:29.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00iyo:',NULL);
INSERT INTO `issues` VALUES (10260,10003,50,'4',NULL,'Augmenter l\'espace disque sur srv-web-02',NULL,NULL,'6','3','1',12,22,12,NULL,'2026-08-08 12:10:43.000','2026-09-18 16:26:57.000','2026-08-16 16:20:28.000',NULL,36000,18000,18000,0,'0|i02y9s:',NULL);
INSERT INTO `issues` VALUES (10261,10003,51,'3',NULL,'Augmenter l\'espace disque sur proxy-01','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Les tests de bout en bout couvrent le cas nominal',NULL,'5','2','1',19,23,19,NULL,'2026-08-08 13:28:15.000','2026-09-22 07:28:26.000','2026-08-12 09:28:28.000',NULL,NULL,NULL,NULL,0,'0|i02ayo:',NULL);
INSERT INTO `issues` VALUES (10262,10003,52,'1',NULL,'Sauvegarde nocturne en échec sur srv-batch-01','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\n*Étapes pour reproduire :*\n# Changer la langue en anglais\n# Se connecter avec un compte client\n# Rafraîchir la page\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}','Recette — https://recette.exemple.fr','6','3','3',22,22,1,NULL,'2026-08-11 15:31:01.000','2026-09-20 10:42:49.000','2026-08-19 07:17:39.000',NULL,NULL,NULL,NULL,0,'0|i02yo0:',NULL);
INSERT INTO `issues` VALUES (10263,10001,79,'3',NULL,'Tester l\'export CSV des factures','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal',NULL,'3','3',NULL,18,6,18,NULL,'2026-08-13 15:59:33.000','2026-09-12 09:08:43.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01clc:',10196);
INSERT INTO `issues` VALUES (10264,10003,53,'3',NULL,'Augmenter l\'espace disque sur srv-db-01','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Les droits existants sont respectés',NULL,'1','4',NULL,2,NULL,2,NULL,'2026-08-14 16:54:52.000','2026-08-24 17:59:33.000',NULL,'2026-10-01',43200,43200,NULL,0,'0|i01ssg:',NULL);
INSERT INTO `issues` VALUES (10265,10000,108,'3',NULL,'Documenter le système de notifications','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Un message clair s\'affiche en cas d\'erreur\n* La page répond en moins d\'une seconde\n\nVoir [le ticket éditeur|https://support.exemple.com/tickets/88412]. [~thomas.moreau] peux-tu valider ?',NULL,'10000','4',NULL,9,17,9,NULL,'2026-08-21 08:50:51.000','2026-09-14 13:22:12.000',NULL,'2026-10-01',NULL,NULL,40500,0,'0|i01t6o:',NULL);
INSERT INTO `issues` VALUES (10266,10000,109,'5',10000,'Déployer en préproduction','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Un message clair s\'affiche en cas d\'erreur\n* Les tests de bout en bout couvrent le cas nominal\n\nLié à DEMO-22. [~ines.david] peux-tu valider ?',NULL,'10001','3','2',16,NULL,1,NULL,'2026-08-22 08:34:38.000','2026-10-01 16:00:00.000','2026-09-05 16:12:07.000',NULL,NULL,NULL,NULL,0,'0|i02z28:',NULL);
INSERT INTO `issues` VALUES (10267,10003,54,'3',NULL,'Augmenter l\'espace disque sur srv-web-01',NULL,NULL,'3','3',NULL,15,21,15,NULL,'2026-08-22 12:41:04.000','2026-09-26 07:15:39.000',NULL,NULL,18000,18000,NULL,0,'0|i01czk:',NULL);
INSERT INTO `issues` VALUES (10268,10002,27,'5',10013,'Préparer la recette','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n* Les droits existants sont respectés\n\nLié à RH-10.',NULL,'6','4','1',10,10,1,NULL,'2026-08-26 11:14:41.000','2026-09-17 06:40:19.000','2026-09-04 15:56:25.000',NULL,NULL,NULL,NULL,0,'0|i036yo:',NULL);
INSERT INTO `issues` VALUES (10269,10001,80,'10001',NULL,'En tant que agent du support, je veux recevoir une alerte de stock','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Les droits existants sont respectés\n* Le résultat est identique sur mobile et sur ordinateur\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}\n\nLié à WEB-18.',NULL,'10001','3','10000',9,NULL,9,NULL,'2026-08-26 11:43:55.000','2026-09-07 06:21:14.000','2026-09-07 06:21:14.000',NULL,NULL,NULL,NULL,0,'0|i02zgg:',NULL);
INSERT INTO `issues` VALUES (10270,10002,28,'3',NULL,'Publier l\'offre de poste « Développeur Go »','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Le résultat est identique sur mobile et sur ordinateur\n* Les droits existants sont respectés',NULL,'5','5','1',14,10,1,NULL,'2026-08-27 10:24:36.000','2026-09-14 14:25:19.000','2026-09-03 17:47:57.000','2026-09-03',NULL,NULL,NULL,0,'0|i039q8:',NULL);
INSERT INTO `issues` VALUES (10271,10000,110,'1',NULL,'La synchronisation mobile affiche une date décalée d\'une heure','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\n*Étapes pour reproduire :*\n# Lancer l\'export\n# Se connecter avec un compte client\n# Changer la langue en anglais\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n[~marie.durand] peux-tu valider ?','Chrome 129 / Windows 11','10001','2','3',9,13,9,1,'2026-08-28 14:15:56.000','2026-09-18 14:48:29.000','2026-09-16 14:22:14.000','2026-09-27',82800,82800,NULL,0,'0|i02bcw:',10105);
INSERT INTO `issues` VALUES (10272,10001,81,'3',NULL,'Refondre la page d\'accueil','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Le résultat est identique sur mobile et sur ordinateur\n* Les droits existants sont respectés\n\n[~marie.durand] peux-tu valider ?',NULL,'3','3',NULL,3,16,3,NULL,'2026-08-28 15:07:51.000','2026-09-09 13:52:10.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01dds:',NULL);
INSERT INTO `issues` VALUES (10273,10001,82,'10001',NULL,'En tant que administrateur, je veux recevoir une alerte de stock','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les droits existants sont respectés',NULL,'3','2',NULL,18,17,1,NULL,'2026-08-28 16:57:47.000','2026-09-07 08:51:14.000',NULL,NULL,NULL,NULL,NULL,0,'0|i00jcw:',NULL);
INSERT INTO `issues` VALUES (10274,10001,83,'3',NULL,'Refondre le système de notifications','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Le résultat est identique sur mobile et sur ordinateur\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}\n\nVoir [la maquette|https://maquettes.exemple.fr/panier].',NULL,'10000','3',NULL,19,NULL,19,NULL,'2026-08-29 09:54:53.000','2026-08-29 09:54:53.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01ds0:',NULL);
INSERT INTO `issues` VALUES (10275,10003,55,'5',10043,'Mettre à jour la documentation','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Les droits existants sont respectés\n* La page répond en moins d\'une seconde\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}',NULL,'1','5',NULL,7,23,7,NULL,'2026-08-29 17:05:54.000','2026-10-01 16:00:00.000',NULL,NULL,18000,0,49500,0,'0|i01wcg:',NULL);
INSERT INTO `issues` VALUES (10276,10000,111,'1',NULL,'Le calendrier des livraisons ignore les caractères accentués','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\n*Étapes pour reproduire :*\n# Ouvrir le tableau de bord\n# Lancer l\'export\n# Rafraîchir la page\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\n[~marie.durand] peux-tu valider ?','Firefox 131 / macOS 15','10001','3','3',11,6,1,NULL,'2026-08-30 16:16:26.000','2026-09-09 07:20:25.000','2026-09-08 06:27:02.000',NULL,118800,118800,NULL,0,'0|i02zuo:',NULL);
INSERT INTO `issues` VALUES (10277,10003,56,'1',NULL,'Redémarrage inopiné de srv-web-02','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\n*Étapes pour reproduire :*\n# Lancer l\'export\n# Se connecter avec un compte client\n# Changer la langue en anglais\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.',NULL,'5','3','4',11,21,1,NULL,'2026-09-01 09:47:58.000','2026-09-28 14:17:14.000','2026-09-13 09:13:09.000',NULL,NULL,NULL,NULL,0,'0|i0308w:',NULL);
INSERT INTO `issues` VALUES (10278,10002,29,'5',10028,'Préparer la recette','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Le résultat est identique sur mobile et sur ordinateur\n\nLié à RH-14. [~camille.petit] peux-tu valider ?',NULL,'5','2','1',1,8,1,NULL,'2026-09-01 12:34:23.000','2026-09-20 16:09:55.000','2026-09-20 16:09:55.000','2026-09-18',NULL,NULL,NULL,0,'0|i02br4:',NULL);
INSERT INTO `issues` VALUES (10279,10000,112,'10001',NULL,'En tant que administrateur, je veux exporter mes factures',NULL,NULL,'10001','4','10000',11,2,11,NULL,'2026-09-02 16:22:29.000','2026-09-20 16:00:45.000','2026-09-13 09:22:04.000',NULL,NULL,NULL,NULL,0,'0|i037cw:',NULL);
INSERT INTO `issues` VALUES (10280,10000,113,'10001',NULL,'En tant que client, je veux partager un panier','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Les droits existants sont respectés\n* Le résultat est identique sur mobile et sur ordinateur\n\nVoir [la maquette|https://maquettes.exemple.fr/panier].',NULL,'10001','3','10000',17,6,17,NULL,'2026-09-04 08:04:57.000','2026-09-19 07:55:26.000','2026-09-19 07:55:26.000',NULL,39600,39600,NULL,0,'0|i030n4:',NULL);
INSERT INTO `issues` VALUES (10281,10001,84,'10001',NULL,'En tant que commercial, je veux filtrer les commandes par statut','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur',NULL,'10001','4','10000',11,16,11,NULL,'2026-09-05 11:24:53.000','2026-10-01 16:00:00.000','2026-09-20 11:22:19.000',NULL,NULL,NULL,NULL,0,'0|i037r4:',10183);
INSERT INTO `issues` VALUES (10282,10000,114,'10000',NULL,'Nouveau back-office','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Les droits existants sont respectés\n* Le résultat est identique sur mobile et sur ordinateur',NULL,'10000','3',NULL,18,2,18,NULL,'2026-09-08 17:55:06.000','2026-10-01 16:00:00.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01e68:',NULL);
INSERT INTO `issues` VALUES (10283,10003,57,'3',NULL,'Ouvrir le flux réseau entre srv-db-02 et le VPN','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Un message clair s\'affiche en cas d\'erreur\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}\n\n[~nicolas.laurent] peux-tu valider ?',NULL,'1','3',NULL,11,21,11,NULL,'2026-09-09 10:00:28.000','2026-10-01 16:00:03.000',NULL,NULL,3600,3600,NULL,0,'0|i01ekg:',NULL);
INSERT INTO `issues` VALUES (10284,10000,115,'3',NULL,'Migrer le tableau des commandes','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Les tests de bout en bout couvrent le cas nominal\n\n!maquette.jpg!',NULL,'10000','4',NULL,15,16,15,2,'2026-09-10 12:08:55.000','2026-09-25 09:48:18.000',NULL,NULL,NULL,NULL,NULL,0,'0|i01tkw:',NULL);
INSERT INTO `issues` VALUES (10285,10003,58,'3',NULL,'Renouveler le certificat TLS de srv-batch-01','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Les tests de bout en bout couvrent le cas nominal\n* Un message clair s\'affiche en cas d\'erreur',NULL,'5','2','1',23,15,23,NULL,'2026-09-10 13:08:16.000','2026-09-26 14:53:37.000','2026-09-26 14:53:37.000',NULL,129600,129600,NULL,0,'0|i02c5c:',NULL);
INSERT INTO `issues` VALUES (10286,10001,85,'10001',NULL,'En tant que administrateur, je veux réinitialiser mon mot de passe','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Le résultat est identique sur mobile et sur ordinateur\n* Les droits existants sont respectés\n\nVoir [la maquette|https://maquettes.exemple.fr/panier]. Lié à WEB-63.',NULL,'10001','2','10000',17,16,1,NULL,'2026-09-11 08:31:25.000','2026-09-29 06:05:08.000','2026-09-29 06:05:08.000',NULL,NULL,NULL,NULL,0,'0|i02cjk:',10183);
INSERT INTO `issues` VALUES (10287,10000,116,'10001',NULL,'En tant que commercial, je veux voir l\'historique de mes demandes','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Les droits existants sont respectés\n* Les tests de bout en bout couvrent le cas nominal',NULL,'10000','3',NULL,6,NULL,6,NULL,'2026-09-11 10:00:01.000','2026-09-28 17:15:06.000',NULL,NULL,72000,0,72000,0,'0|i01eyo:',NULL);
INSERT INTO `issues` VALUES (10288,10001,86,'1',NULL,'La recherche avancée envoie deux fois le même message','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\n*Étapes pour reproduire :*\n# Ouvrir le tableau de bord\n# Changer la langue en anglais\n# Ajouter deux articles au panier\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.','Recette — https://recette.exemple.fr','10001','2','10000',19,3,19,NULL,'2026-09-13 12:56:54.000','2026-09-30 16:18:17.000','2026-09-25 11:01:35.000',NULL,118800,118800,NULL,0,'0|i02cxs:',NULL);
INSERT INTO `issues` VALUES (10289,10000,117,'10001',NULL,'En tant que client, je veux recevoir une alerte de stock','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Le résultat est identique sur mobile et sur ordinateur\n* Un message clair s\'affiche en cas d\'erreur\n* La page répond en moins d\'une seconde\n\n{noformat}\n2026-03-02 10:14:07 ERROR [http-exec-4] PanierController : panier introuvable (session expirée)\njava.lang.IllegalStateException: panier absent\n{noformat}\n\nVoir [la maquette|https://maquettes.exemple.fr/panier]. Lié à DEMO-7. [~hugo.robert] peux-tu valider ?',NULL,'10001','1','10000',20,2,20,NULL,'2026-09-15 17:56:02.000','2026-10-01 16:00:02.000','2026-09-27 16:50:44.000',NULL,46800,46800,NULL,0,'0|i02134:',10003);
INSERT INTO `issues` VALUES (10290,10001,87,'1',NULL,'La synchronisation mobile s\'affiche mal sur mobile','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\n*Étapes pour reproduire :*\n# Ajouter deux articles au panier\n# Rafraîchir la page\n# Ouvrir le tableau de bord\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\nVoir [la maquette|https://maquettes.exemple.fr/panier]. Lié à WEB-46. [~camille.petit] peux-tu valider ?','Production — srv-web-01','10000','3',NULL,4,17,4,NULL,'2026-09-17 07:13:13.000','2026-10-01 16:00:01.000',NULL,NULL,NULL,NULL,15300,0,'0|i01fcw:',NULL);
INSERT INTO `issues` VALUES (10291,10001,88,'10001',NULL,'En tant que administrateur, je veux recevoir une alerte de stock','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Le résultat est identique sur mobile et sur ordinateur\n* La page répond en moins d\'une seconde\n\n{code:java}\npublic BigDecimal total(Cart cart) {\n    return cart.lines().stream()\n        .map(Line::amount)\n        .reduce(BigDecimal.ZERO, BigDecimal::add);\n}\n{code}\n\n[~ines.david] peux-tu valider ?',NULL,'10001','3','10000',11,4,11,NULL,'2026-09-19 14:20:07.000','2026-10-01 16:00:01.000','2026-10-01 16:00:01.000','2026-10-18',28800,28800,NULL,0,'0|i0311c:',10183);
INSERT INTO `issues` VALUES (10292,10001,89,'1',NULL,'Le formulaire d\'inscription est très lent au-delà de 1 000 lignes','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\n*Étapes pour reproduire :*\n# Changer la langue en anglais\n# Ajouter deux articles au panier\n# Se connecter avec un compte client\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.','Safari 18 / iOS 18','3','3',NULL,13,15,13,NULL,'2026-09-20 14:22:21.000','2026-10-01 16:00:02.000',NULL,'2026-10-18',NULL,NULL,NULL,0,'0|i01fr4:',NULL);
INSERT INTO `issues` VALUES (10293,10003,59,'3',NULL,'Redémarrage inopiné de srv-batch-01','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Le résultat est identique sur mobile et sur ordinateur',NULL,'6','4','1',23,21,23,NULL,'2026-09-21 12:58:48.000','2026-10-01 16:00:01.000','2026-10-01 16:00:00.000','2026-10-26',86400,35100,51300,0,'0|i0385c:',NULL);
INSERT INTO `issues` VALUES (10294,10001,90,'5',10111,'Mettre à jour la documentation','h3. Contexte\nLa supervision a levé l\'alerte trois nuits de suite.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Un message clair s\'affiche en cas d\'erreur\n\n{noformat}\nmars 12 03:00:01 srv-db-02 backup.sh[2214]: pg_dump: erreur : connexion refusée\n{noformat}\n\nLié à WEB-35.',NULL,'10001','2','10000',12,NULL,12,NULL,'2026-09-22 08:42:26.000','2026-10-01 16:00:04.000','2026-10-01 16:00:02.000',NULL,111600,111600,NULL,0,'0|i02dc0:',NULL);
INSERT INTO `issues` VALUES (10295,10000,118,'10001',NULL,'En tant que administrateur, je veux partager un panier','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* Un message clair s\'affiche en cas d\'erreur\n* Le résultat est identique sur mobile et sur ordinateur\n\nVoir [la documentation interne|https://wiki.exemple.fr/exploitation/sauvegardes].',NULL,'10001','4','10000',6,2,6,NULL,'2026-09-25 07:00:28.000','2026-10-01 16:00:01.000','2026-10-01 16:00:01.000','2026-11-23',NULL,NULL,NULL,0,'0|i038jk:',NULL);
INSERT INTO `issues` VALUES (10296,10003,60,'5',10177,'Déployer en préproduction','h3. Contexte\nLe comportement actuel oblige à refaire la saisie, ce qui fait perdre du temps à tout le monde.\n\nh3. Critères d\'acceptation\n* Les tests de bout en bout couvrent le cas nominal\n* La page répond en moins d\'une seconde\n\nLié à OPS-52.',NULL,'3','2',NULL,15,22,15,NULL,'2026-09-25 10:46:46.000','2026-10-01 16:00:01.000',NULL,'2026-11-12',NULL,NULL,21600,0,'0|i00jr4:',NULL);
INSERT INTO `issues` VALUES (10297,10000,119,'1',NULL,'Le panier est très lent au-delà de 1 000 lignes','h3. Contexte\nLe besoin vient de l\'équipe commerciale, qui doit pouvoir répondre sans passer par le support.\n\n*Étapes pour reproduire :*\n# Rafraîchir la page\n# Ajouter deux articles au panier\n# Se connecter avec un compte client\n\n*Résultat attendu :* tout fonctionne comme avant.\n*Résultat obtenu :* _le traitement s\'arrête sans message_.\n\nLié à DEMO-35.','Production — srv-web-01','10001','5','2',16,5,16,NULL,'2026-09-27 10:26:03.000','2026-10-01 16:00:01.000','2026-10-01 16:00:01.000','2026-10-29',NULL,NULL,9900,0,'0|i03a4g:',10003);
INSERT INTO `issues` VALUES (10298,10000,120,'5',10060,'Rédiger les tests unitaires','h3. Contexte\nPlusieurs clients nous ont remonté le problème depuis la dernière mise en production.\n\nh3. Critères d\'acceptation\n* Les droits existants sont respectés\n* Le résultat est identique sur mobile et sur ordinateur\n* Un message clair s\'affiche en cas d\'erreur\n\n{code}\ncurl -s https://api.exemple.fr/v1/factures?client=42 | jq \'.[0]\'\n{code}\n\n[~marie.durand] peux-tu valider ?',NULL,'10001','3','10000',11,17,11,NULL,'2026-09-29 11:13:18.000','2026-10-01 16:00:02.000','2026-10-01 16:00:02.000',NULL,NULL,NULL,NULL,0,'0|i031fk:',NULL);
INSERT INTO `issues` VALUES (10299,10002,30,'3',NULL,'Planifier la formation sécurité','h3. Contexte\nC\'est un prérequis pour la prochaine version, à traiter avant la fin du mois.\n\nh3. Critères d\'acceptation\n* La page répond en moins d\'une seconde\n* Le résultat est identique sur mobile et sur ordinateur',NULL,'5','3','1',1,NULL,1,NULL,'2026-09-30 10:58:12.000','2026-10-01 16:00:01.000','2026-10-01 16:00:01.000','2026-11-23',NULL,NULL,26100,0,'0|i031ts:',NULL);
/*!40000 ALTER TABLE `issues` ENABLE KEYS */;
DROP TABLE IF EXISTS `labels`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `labels` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `issue_id` bigint(20) NOT NULL,
  `field_id` bigint(20) DEFAULT NULL,
  `label` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_labels_issue` (`issue_id`),
  KEY `ix_labels_label` (`label`),
  KEY `fk_labels_field` (`field_id`),
  CONSTRAINT `fk_labels_field` FOREIGN KEY (`field_id`) REFERENCES `custom_fields` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_labels_issue` FOREIGN KEY (`issue_id`) REFERENCES `issues` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=443 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `labels` DISABLE KEYS */;
INSERT INTO `labels` VALUES (1,10000,10109,'facturation');
INSERT INTO `labels` VALUES (2,10000,10109,'onboarding');
INSERT INTO `labels` VALUES (3,10001,NULL,'client');
INSERT INTO `labels` VALUES (4,10001,NULL,'dette-technique');
INSERT INTO `labels` VALUES (5,10001,NULL,'à-valider');
INSERT INTO `labels` VALUES (6,10003,10109,'facturation');
INSERT INTO `labels` VALUES (7,10003,10109,'accessibilité');
INSERT INTO `labels` VALUES (8,10005,NULL,'backend');
INSERT INTO `labels` VALUES (9,10005,NULL,'mobile');
INSERT INTO `labels` VALUES (10,10005,NULL,'ux');
INSERT INTO `labels` VALUES (11,10006,NULL,'mobile');
INSERT INTO `labels` VALUES (12,10006,NULL,'client');
INSERT INTO `labels` VALUES (13,10006,NULL,'dette-technique');
INSERT INTO `labels` VALUES (14,10007,NULL,'sécurité');
INSERT INTO `labels` VALUES (15,10007,NULL,'frontend');
INSERT INTO `labels` VALUES (16,10009,NULL,'sécurité');
INSERT INTO `labels` VALUES (17,10009,NULL,'régression');
INSERT INTO `labels` VALUES (18,10009,NULL,'ux');
INSERT INTO `labels` VALUES (19,10010,NULL,'à-valider');
INSERT INTO `labels` VALUES (20,10010,NULL,'backend');
INSERT INTO `labels` VALUES (21,10013,10109,'accessibilité');
INSERT INTO `labels` VALUES (22,10013,10109,'facturation');
INSERT INTO `labels` VALUES (23,10015,NULL,'perf');
INSERT INTO `labels` VALUES (24,10015,NULL,'régression');
INSERT INTO `labels` VALUES (25,10015,NULL,'mobile');
INSERT INTO `labels` VALUES (26,10017,NULL,'sécurité');
INSERT INTO `labels` VALUES (27,10019,NULL,'urgent');
INSERT INTO `labels` VALUES (28,10019,NULL,'perf');
INSERT INTO `labels` VALUES (29,10023,NULL,'dette-technique');
INSERT INTO `labels` VALUES (30,10023,NULL,'backend');
INSERT INTO `labels` VALUES (31,10023,NULL,'client');
INSERT INTO `labels` VALUES (32,10024,NULL,'backend');
INSERT INTO `labels` VALUES (33,10024,NULL,'urgent');
INSERT INTO `labels` VALUES (34,10025,NULL,'à-valider');
INSERT INTO `labels` VALUES (35,10025,NULL,'régression');
INSERT INTO `labels` VALUES (36,10028,NULL,'à-valider');
INSERT INTO `labels` VALUES (37,10028,NULL,'ux');
INSERT INTO `labels` VALUES (38,10029,NULL,'sécurité');
INSERT INTO `labels` VALUES (39,10029,NULL,'frontend');
INSERT INTO `labels` VALUES (40,10029,NULL,'à-valider');
INSERT INTO `labels` VALUES (41,10031,NULL,'frontend');
INSERT INTO `labels` VALUES (42,10031,NULL,'à-valider');
INSERT INTO `labels` VALUES (43,10031,NULL,'client');
INSERT INTO `labels` VALUES (44,10032,NULL,'backend');
INSERT INTO `labels` VALUES (45,10032,NULL,'sécurité');
INSERT INTO `labels` VALUES (46,10032,NULL,'à-valider');
INSERT INTO `labels` VALUES (47,10033,NULL,'dette-technique');
INSERT INTO `labels` VALUES (48,10033,10109,'accessibilité');
INSERT INTO `labels` VALUES (49,10033,10109,'rgpd');
INSERT INTO `labels` VALUES (50,10034,NULL,'perf');
INSERT INTO `labels` VALUES (51,10034,NULL,'urgent');
INSERT INTO `labels` VALUES (52,10034,NULL,'ux');
INSERT INTO `labels` VALUES (53,10035,10109,'facturation');
INSERT INTO `labels` VALUES (54,10035,10109,'accessibilité');
INSERT INTO `labels` VALUES (55,10038,NULL,'mobile');
INSERT INTO `labels` VALUES (56,10039,NULL,'régression');
INSERT INTO `labels` VALUES (57,10039,NULL,'à-valider');
INSERT INTO `labels` VALUES (58,10040,NULL,'sécurité');
INSERT INTO `labels` VALUES (59,10040,NULL,'frontend');
INSERT INTO `labels` VALUES (60,10041,NULL,'frontend');
INSERT INTO `labels` VALUES (61,10041,NULL,'urgent');
INSERT INTO `labels` VALUES (62,10041,NULL,'sécurité');
INSERT INTO `labels` VALUES (63,10042,NULL,'régression');
INSERT INTO `labels` VALUES (64,10042,NULL,'sécurité');
INSERT INTO `labels` VALUES (65,10042,10109,'facturation');
INSERT INTO `labels` VALUES (66,10042,10109,'onboarding');
INSERT INTO `labels` VALUES (67,10043,NULL,'perf');
INSERT INTO `labels` VALUES (68,10043,NULL,'ux');
INSERT INTO `labels` VALUES (69,10044,NULL,'backend');
INSERT INTO `labels` VALUES (70,10044,NULL,'perf');
INSERT INTO `labels` VALUES (71,10044,NULL,'régression');
INSERT INTO `labels` VALUES (72,10047,10109,'facturation');
INSERT INTO `labels` VALUES (73,10047,10109,'rgpd');
INSERT INTO `labels` VALUES (74,10048,NULL,'client');
INSERT INTO `labels` VALUES (75,10048,10109,'onboarding');
INSERT INTO `labels` VALUES (76,10048,10109,'rgpd');
INSERT INTO `labels` VALUES (77,10049,NULL,'client');
INSERT INTO `labels` VALUES (78,10049,NULL,'à-valider');
INSERT INTO `labels` VALUES (79,10050,NULL,'client');
INSERT INTO `labels` VALUES (80,10050,NULL,'urgent');
INSERT INTO `labels` VALUES (81,10051,NULL,'urgent');
INSERT INTO `labels` VALUES (82,10052,NULL,'mobile');
INSERT INTO `labels` VALUES (83,10053,NULL,'mobile');
INSERT INTO `labels` VALUES (84,10054,NULL,'client');
INSERT INTO `labels` VALUES (85,10055,NULL,'perf');
INSERT INTO `labels` VALUES (86,10055,NULL,'backend');
INSERT INTO `labels` VALUES (87,10059,NULL,'à-valider');
INSERT INTO `labels` VALUES (88,10059,NULL,'backend');
INSERT INTO `labels` VALUES (89,10059,10109,'onboarding');
INSERT INTO `labels` VALUES (90,10060,NULL,'frontend');
INSERT INTO `labels` VALUES (91,10060,NULL,'backend');
INSERT INTO `labels` VALUES (92,10060,NULL,'urgent');
INSERT INTO `labels` VALUES (93,10063,NULL,'mobile');
INSERT INTO `labels` VALUES (94,10063,NULL,'backend');
INSERT INTO `labels` VALUES (95,10063,NULL,'à-valider');
INSERT INTO `labels` VALUES (96,10065,NULL,'frontend');
INSERT INTO `labels` VALUES (97,10065,NULL,'ux');
INSERT INTO `labels` VALUES (98,10068,NULL,'client');
INSERT INTO `labels` VALUES (99,10068,NULL,'à-valider');
INSERT INTO `labels` VALUES (100,10068,10109,'onboarding');
INSERT INTO `labels` VALUES (101,10068,10109,'facturation');
INSERT INTO `labels` VALUES (102,10070,NULL,'perf');
INSERT INTO `labels` VALUES (103,10070,NULL,'frontend');
INSERT INTO `labels` VALUES (104,10070,NULL,'régression');
INSERT INTO `labels` VALUES (105,10070,10109,'accessibilité');
INSERT INTO `labels` VALUES (106,10070,10109,'facturation');
INSERT INTO `labels` VALUES (107,10072,NULL,'ux');
INSERT INTO `labels` VALUES (108,10074,NULL,'frontend');
INSERT INTO `labels` VALUES (109,10074,NULL,'perf');
INSERT INTO `labels` VALUES (110,10074,NULL,'urgent');
INSERT INTO `labels` VALUES (111,10075,NULL,'régression');
INSERT INTO `labels` VALUES (112,10075,NULL,'client');
INSERT INTO `labels` VALUES (113,10075,NULL,'à-valider');
INSERT INTO `labels` VALUES (114,10076,NULL,'à-valider');
INSERT INTO `labels` VALUES (115,10076,NULL,'sécurité');
INSERT INTO `labels` VALUES (116,10076,NULL,'ux');
INSERT INTO `labels` VALUES (117,10078,NULL,'perf');
INSERT INTO `labels` VALUES (118,10078,10109,'onboarding');
INSERT INTO `labels` VALUES (119,10079,NULL,'backend');
INSERT INTO `labels` VALUES (120,10079,NULL,'sécurité');
INSERT INTO `labels` VALUES (121,10079,NULL,'frontend');
INSERT INTO `labels` VALUES (122,10081,NULL,'à-valider');
INSERT INTO `labels` VALUES (123,10081,NULL,'urgent');
INSERT INTO `labels` VALUES (124,10081,NULL,'client');
INSERT INTO `labels` VALUES (125,10081,10109,'facturation');
INSERT INTO `labels` VALUES (126,10081,10109,'accessibilité');
INSERT INTO `labels` VALUES (127,10082,NULL,'à-valider');
INSERT INTO `labels` VALUES (128,10084,NULL,'frontend');
INSERT INTO `labels` VALUES (129,10084,NULL,'urgent');
INSERT INTO `labels` VALUES (130,10084,NULL,'mobile');
INSERT INTO `labels` VALUES (131,10085,NULL,'dette-technique');
INSERT INTO `labels` VALUES (132,10085,NULL,'régression');
INSERT INTO `labels` VALUES (133,10086,NULL,'mobile');
INSERT INTO `labels` VALUES (134,10087,NULL,'ux');
INSERT INTO `labels` VALUES (135,10087,NULL,'à-valider');
INSERT INTO `labels` VALUES (136,10088,NULL,'à-valider');
INSERT INTO `labels` VALUES (137,10088,NULL,'client');
INSERT INTO `labels` VALUES (138,10088,NULL,'ux');
INSERT INTO `labels` VALUES (139,10089,NULL,'mobile');
INSERT INTO `labels` VALUES (140,10091,NULL,'à-valider');
INSERT INTO `labels` VALUES (141,10091,NULL,'ux');
INSERT INTO `labels` VALUES (142,10092,NULL,'backend');
INSERT INTO `labels` VALUES (143,10092,NULL,'urgent');
INSERT INTO `labels` VALUES (144,10093,NULL,'frontend');
INSERT INTO `labels` VALUES (145,10093,NULL,'à-valider');
INSERT INTO `labels` VALUES (146,10093,NULL,'ux');
INSERT INTO `labels` VALUES (147,10094,NULL,'ux');
INSERT INTO `labels` VALUES (148,10094,NULL,'frontend');
INSERT INTO `labels` VALUES (149,10094,NULL,'mobile');
INSERT INTO `labels` VALUES (150,10095,NULL,'ux');
INSERT INTO `labels` VALUES (151,10095,NULL,'dette-technique');
INSERT INTO `labels` VALUES (152,10095,NULL,'backend');
INSERT INTO `labels` VALUES (153,10097,NULL,'sécurité');
INSERT INTO `labels` VALUES (154,10098,NULL,'urgent');
INSERT INTO `labels` VALUES (155,10098,NULL,'frontend');
INSERT INTO `labels` VALUES (156,10098,10109,'rgpd');
INSERT INTO `labels` VALUES (157,10099,NULL,'frontend');
INSERT INTO `labels` VALUES (158,10100,NULL,'sécurité');
INSERT INTO `labels` VALUES (159,10100,NULL,'backend');
INSERT INTO `labels` VALUES (160,10102,NULL,'sécurité');
INSERT INTO `labels` VALUES (161,10102,NULL,'dette-technique');
INSERT INTO `labels` VALUES (162,10102,NULL,'urgent');
INSERT INTO `labels` VALUES (163,10102,10109,'rgpd');
INSERT INTO `labels` VALUES (164,10103,NULL,'sécurité');
INSERT INTO `labels` VALUES (165,10103,NULL,'frontend');
INSERT INTO `labels` VALUES (166,10104,NULL,'régression');
INSERT INTO `labels` VALUES (167,10104,NULL,'à-valider');
INSERT INTO `labels` VALUES (168,10104,NULL,'sécurité');
INSERT INTO `labels` VALUES (169,10105,NULL,'régression');
INSERT INTO `labels` VALUES (170,10109,NULL,'frontend');
INSERT INTO `labels` VALUES (171,10109,NULL,'client');
INSERT INTO `labels` VALUES (172,10112,NULL,'mobile');
INSERT INTO `labels` VALUES (173,10112,NULL,'frontend');
INSERT INTO `labels` VALUES (174,10113,NULL,'perf');
INSERT INTO `labels` VALUES (175,10113,NULL,'régression');
INSERT INTO `labels` VALUES (176,10113,NULL,'backend');
INSERT INTO `labels` VALUES (177,10114,10109,'facturation');
INSERT INTO `labels` VALUES (178,10114,10109,'rgpd');
INSERT INTO `labels` VALUES (179,10115,NULL,'frontend');
INSERT INTO `labels` VALUES (180,10115,NULL,'mobile');
INSERT INTO `labels` VALUES (181,10117,NULL,'ux');
INSERT INTO `labels` VALUES (182,10117,NULL,'client');
INSERT INTO `labels` VALUES (183,10118,NULL,'ux');
INSERT INTO `labels` VALUES (184,10118,NULL,'sécurité');
INSERT INTO `labels` VALUES (185,10119,10109,'rgpd');
INSERT INTO `labels` VALUES (186,10119,10109,'onboarding');
INSERT INTO `labels` VALUES (187,10120,NULL,'backend');
INSERT INTO `labels` VALUES (188,10120,NULL,'urgent');
INSERT INTO `labels` VALUES (189,10120,NULL,'ux');
INSERT INTO `labels` VALUES (190,10122,10109,'onboarding');
INSERT INTO `labels` VALUES (191,10123,NULL,'mobile');
INSERT INTO `labels` VALUES (192,10123,NULL,'sécurité');
INSERT INTO `labels` VALUES (193,10124,NULL,'à-valider');
INSERT INTO `labels` VALUES (194,10124,NULL,'dette-technique');
INSERT INTO `labels` VALUES (195,10125,NULL,'ux');
INSERT INTO `labels` VALUES (196,10125,NULL,'frontend');
INSERT INTO `labels` VALUES (197,10125,10109,'rgpd');
INSERT INTO `labels` VALUES (198,10127,NULL,'perf');
INSERT INTO `labels` VALUES (199,10127,NULL,'mobile');
INSERT INTO `labels` VALUES (200,10127,10109,'facturation');
INSERT INTO `labels` VALUES (201,10127,10109,'accessibilité');
INSERT INTO `labels` VALUES (202,10128,NULL,'régression');
INSERT INTO `labels` VALUES (203,10128,NULL,'sécurité');
INSERT INTO `labels` VALUES (204,10128,NULL,'perf');
INSERT INTO `labels` VALUES (205,10128,10109,'rgpd');
INSERT INTO `labels` VALUES (206,10130,NULL,'urgent');
INSERT INTO `labels` VALUES (207,10130,NULL,'dette-technique');
INSERT INTO `labels` VALUES (208,10133,NULL,'mobile');
INSERT INTO `labels` VALUES (209,10134,NULL,'ux');
INSERT INTO `labels` VALUES (210,10134,NULL,'régression');
INSERT INTO `labels` VALUES (211,10134,NULL,'backend');
INSERT INTO `labels` VALUES (212,10137,NULL,'urgent');
INSERT INTO `labels` VALUES (213,10138,NULL,'mobile');
INSERT INTO `labels` VALUES (214,10138,NULL,'sécurité');
INSERT INTO `labels` VALUES (215,10138,NULL,'perf');
INSERT INTO `labels` VALUES (216,10139,NULL,'ux');
INSERT INTO `labels` VALUES (217,10140,NULL,'urgent');
INSERT INTO `labels` VALUES (218,10140,NULL,'backend');
INSERT INTO `labels` VALUES (219,10140,NULL,'frontend');
INSERT INTO `labels` VALUES (220,10141,NULL,'backend');
INSERT INTO `labels` VALUES (221,10142,NULL,'frontend');
INSERT INTO `labels` VALUES (222,10143,10109,'accessibilité');
INSERT INTO `labels` VALUES (223,10144,NULL,'client');
INSERT INTO `labels` VALUES (224,10144,NULL,'urgent');
INSERT INTO `labels` VALUES (225,10144,NULL,'à-valider');
INSERT INTO `labels` VALUES (226,10145,NULL,'mobile');
INSERT INTO `labels` VALUES (227,10145,NULL,'à-valider');
INSERT INTO `labels` VALUES (228,10145,NULL,'urgent');
INSERT INTO `labels` VALUES (229,10147,NULL,'backend');
INSERT INTO `labels` VALUES (230,10147,NULL,'urgent');
INSERT INTO `labels` VALUES (231,10149,NULL,'urgent');
INSERT INTO `labels` VALUES (232,10149,NULL,'dette-technique');
INSERT INTO `labels` VALUES (233,10150,10109,'onboarding');
INSERT INTO `labels` VALUES (234,10150,10109,'accessibilité');
INSERT INTO `labels` VALUES (235,10152,10109,'onboarding');
INSERT INTO `labels` VALUES (236,10155,NULL,'régression');
INSERT INTO `labels` VALUES (237,10155,NULL,'urgent');
INSERT INTO `labels` VALUES (238,10156,NULL,'dette-technique');
INSERT INTO `labels` VALUES (239,10156,NULL,'backend');
INSERT INTO `labels` VALUES (240,10157,NULL,'frontend');
INSERT INTO `labels` VALUES (241,10157,NULL,'sécurité');
INSERT INTO `labels` VALUES (242,10157,NULL,'client');
INSERT INTO `labels` VALUES (243,10158,NULL,'sécurité');
INSERT INTO `labels` VALUES (244,10158,NULL,'régression');
INSERT INTO `labels` VALUES (245,10161,10109,'facturation');
INSERT INTO `labels` VALUES (246,10164,NULL,'mobile');
INSERT INTO `labels` VALUES (247,10165,NULL,'mobile');
INSERT INTO `labels` VALUES (248,10165,NULL,'perf');
INSERT INTO `labels` VALUES (249,10165,NULL,'client');
INSERT INTO `labels` VALUES (250,10167,NULL,'perf');
INSERT INTO `labels` VALUES (251,10167,NULL,'à-valider');
INSERT INTO `labels` VALUES (252,10167,NULL,'client');
INSERT INTO `labels` VALUES (253,10168,NULL,'client');
INSERT INTO `labels` VALUES (254,10168,NULL,'sécurité');
INSERT INTO `labels` VALUES (255,10172,NULL,'perf');
INSERT INTO `labels` VALUES (256,10172,NULL,'backend');
INSERT INTO `labels` VALUES (257,10173,NULL,'client');
INSERT INTO `labels` VALUES (258,10173,NULL,'frontend');
INSERT INTO `labels` VALUES (259,10174,NULL,'client');
INSERT INTO `labels` VALUES (260,10174,NULL,'frontend');
INSERT INTO `labels` VALUES (261,10174,NULL,'backend');
INSERT INTO `labels` VALUES (262,10175,NULL,'à-valider');
INSERT INTO `labels` VALUES (263,10175,NULL,'backend');
INSERT INTO `labels` VALUES (264,10176,NULL,'dette-technique');
INSERT INTO `labels` VALUES (265,10176,NULL,'à-valider');
INSERT INTO `labels` VALUES (266,10177,NULL,'urgent');
INSERT INTO `labels` VALUES (267,10177,NULL,'mobile');
INSERT INTO `labels` VALUES (268,10179,NULL,'urgent');
INSERT INTO `labels` VALUES (269,10180,NULL,'dette-technique');
INSERT INTO `labels` VALUES (270,10180,NULL,'à-valider');
INSERT INTO `labels` VALUES (271,10182,NULL,'client');
INSERT INTO `labels` VALUES (272,10182,NULL,'mobile');
INSERT INTO `labels` VALUES (273,10182,NULL,'frontend');
INSERT INTO `labels` VALUES (274,10183,NULL,'frontend');
INSERT INTO `labels` VALUES (275,10183,NULL,'régression');
INSERT INTO `labels` VALUES (276,10183,NULL,'perf');
INSERT INTO `labels` VALUES (277,10184,NULL,'sécurité');
INSERT INTO `labels` VALUES (278,10185,NULL,'ux');
INSERT INTO `labels` VALUES (279,10185,NULL,'frontend');
INSERT INTO `labels` VALUES (280,10186,NULL,'régression');
INSERT INTO `labels` VALUES (281,10186,NULL,'dette-technique');
INSERT INTO `labels` VALUES (282,10187,NULL,'ux');
INSERT INTO `labels` VALUES (283,10187,NULL,'client');
INSERT INTO `labels` VALUES (284,10187,NULL,'à-valider');
INSERT INTO `labels` VALUES (285,10188,NULL,'backend');
INSERT INTO `labels` VALUES (286,10188,NULL,'urgent');
INSERT INTO `labels` VALUES (287,10190,NULL,'dette-technique');
INSERT INTO `labels` VALUES (288,10191,NULL,'sécurité');
INSERT INTO `labels` VALUES (289,10191,NULL,'ux');
INSERT INTO `labels` VALUES (290,10192,NULL,'à-valider');
INSERT INTO `labels` VALUES (291,10192,NULL,'mobile');
INSERT INTO `labels` VALUES (292,10193,NULL,'perf');
INSERT INTO `labels` VALUES (293,10193,NULL,'mobile');
INSERT INTO `labels` VALUES (294,10193,NULL,'ux');
INSERT INTO `labels` VALUES (295,10195,NULL,'mobile');
INSERT INTO `labels` VALUES (296,10195,NULL,'perf');
INSERT INTO `labels` VALUES (297,10196,NULL,'à-valider');
INSERT INTO `labels` VALUES (298,10196,NULL,'client');
INSERT INTO `labels` VALUES (299,10196,NULL,'backend');
INSERT INTO `labels` VALUES (300,10197,NULL,'perf');
INSERT INTO `labels` VALUES (301,10197,NULL,'frontend');
INSERT INTO `labels` VALUES (302,10197,NULL,'client');
INSERT INTO `labels` VALUES (303,10197,10109,'onboarding');
INSERT INTO `labels` VALUES (304,10197,10109,'facturation');
INSERT INTO `labels` VALUES (305,10200,NULL,'dette-technique');
INSERT INTO `labels` VALUES (306,10202,NULL,'urgent');
INSERT INTO `labels` VALUES (307,10204,NULL,'mobile');
INSERT INTO `labels` VALUES (308,10204,NULL,'frontend');
INSERT INTO `labels` VALUES (309,10207,NULL,'régression');
INSERT INTO `labels` VALUES (310,10208,NULL,'régression');
INSERT INTO `labels` VALUES (311,10208,NULL,'frontend');
INSERT INTO `labels` VALUES (312,10209,NULL,'régression');
INSERT INTO `labels` VALUES (313,10211,NULL,'perf');
INSERT INTO `labels` VALUES (314,10211,NULL,'régression');
INSERT INTO `labels` VALUES (315,10211,10109,'rgpd');
INSERT INTO `labels` VALUES (316,10212,NULL,'client');
INSERT INTO `labels` VALUES (317,10212,NULL,'backend');
INSERT INTO `labels` VALUES (318,10212,NULL,'dette-technique');
INSERT INTO `labels` VALUES (319,10213,NULL,'client');
INSERT INTO `labels` VALUES (320,10213,NULL,'dette-technique');
INSERT INTO `labels` VALUES (321,10213,NULL,'à-valider');
INSERT INTO `labels` VALUES (322,10214,NULL,'régression');
INSERT INTO `labels` VALUES (323,10214,NULL,'mobile');
INSERT INTO `labels` VALUES (324,10214,10109,'onboarding');
INSERT INTO `labels` VALUES (325,10215,NULL,'mobile');
INSERT INTO `labels` VALUES (326,10215,10109,'onboarding');
INSERT INTO `labels` VALUES (327,10217,10109,'onboarding');
INSERT INTO `labels` VALUES (328,10217,10109,'accessibilité');
INSERT INTO `labels` VALUES (329,10218,NULL,'urgent');
INSERT INTO `labels` VALUES (330,10218,NULL,'perf');
INSERT INTO `labels` VALUES (331,10218,NULL,'ux');
INSERT INTO `labels` VALUES (332,10221,NULL,'backend');
INSERT INTO `labels` VALUES (333,10221,10109,'facturation');
INSERT INTO `labels` VALUES (334,10222,NULL,'client');
INSERT INTO `labels` VALUES (335,10223,NULL,'perf');
INSERT INTO `labels` VALUES (336,10223,NULL,'régression');
INSERT INTO `labels` VALUES (337,10223,NULL,'sécurité');
INSERT INTO `labels` VALUES (338,10224,NULL,'backend');
INSERT INTO `labels` VALUES (339,10224,NULL,'frontend');
INSERT INTO `labels` VALUES (340,10224,NULL,'à-valider');
INSERT INTO `labels` VALUES (341,10224,10109,'onboarding');
INSERT INTO `labels` VALUES (342,10224,10109,'rgpd');
INSERT INTO `labels` VALUES (343,10225,NULL,'mobile');
INSERT INTO `labels` VALUES (344,10225,NULL,'à-valider');
INSERT INTO `labels` VALUES (345,10227,NULL,'dette-technique');
INSERT INTO `labels` VALUES (346,10227,10109,'onboarding');
INSERT INTO `labels` VALUES (347,10229,NULL,'urgent');
INSERT INTO `labels` VALUES (348,10229,NULL,'dette-technique');
INSERT INTO `labels` VALUES (349,10229,10109,'accessibilité');
INSERT INTO `labels` VALUES (350,10230,NULL,'urgent');
INSERT INTO `labels` VALUES (351,10230,NULL,'régression');
INSERT INTO `labels` VALUES (352,10232,NULL,'régression');
INSERT INTO `labels` VALUES (353,10232,NULL,'client');
INSERT INTO `labels` VALUES (354,10232,NULL,'sécurité');
INSERT INTO `labels` VALUES (355,10233,NULL,'sécurité');
INSERT INTO `labels` VALUES (356,10235,NULL,'régression');
INSERT INTO `labels` VALUES (357,10236,10109,'facturation');
INSERT INTO `labels` VALUES (358,10236,10109,'accessibilité');
INSERT INTO `labels` VALUES (359,10237,NULL,'dette-technique');
INSERT INTO `labels` VALUES (360,10238,NULL,'sécurité');
INSERT INTO `labels` VALUES (361,10238,NULL,'client');
INSERT INTO `labels` VALUES (362,10238,NULL,'régression');
INSERT INTO `labels` VALUES (363,10239,NULL,'régression');
INSERT INTO `labels` VALUES (364,10239,NULL,'client');
INSERT INTO `labels` VALUES (365,10240,NULL,'à-valider');
INSERT INTO `labels` VALUES (366,10240,NULL,'mobile');
INSERT INTO `labels` VALUES (367,10240,NULL,'perf');
INSERT INTO `labels` VALUES (368,10240,10109,'onboarding');
INSERT INTO `labels` VALUES (369,10240,10109,'rgpd');
INSERT INTO `labels` VALUES (370,10242,NULL,'sécurité');
INSERT INTO `labels` VALUES (371,10242,NULL,'régression');
INSERT INTO `labels` VALUES (372,10243,NULL,'régression');
INSERT INTO `labels` VALUES (373,10244,10109,'rgpd');
INSERT INTO `labels` VALUES (374,10244,10109,'facturation');
INSERT INTO `labels` VALUES (375,10245,NULL,'backend');
INSERT INTO `labels` VALUES (376,10245,NULL,'perf');
INSERT INTO `labels` VALUES (377,10247,NULL,'sécurité');
INSERT INTO `labels` VALUES (378,10247,NULL,'à-valider');
INSERT INTO `labels` VALUES (379,10247,NULL,'perf');
INSERT INTO `labels` VALUES (380,10248,NULL,'frontend');
INSERT INTO `labels` VALUES (381,10249,NULL,'backend');
INSERT INTO `labels` VALUES (382,10249,10109,'rgpd');
INSERT INTO `labels` VALUES (383,10249,10109,'facturation');
INSERT INTO `labels` VALUES (384,10251,NULL,'dette-technique');
INSERT INTO `labels` VALUES (385,10251,NULL,'à-valider');
INSERT INTO `labels` VALUES (386,10252,NULL,'dette-technique');
INSERT INTO `labels` VALUES (387,10252,NULL,'à-valider');
INSERT INTO `labels` VALUES (388,10253,NULL,'régression');
INSERT INTO `labels` VALUES (389,10254,NULL,'ux');
INSERT INTO `labels` VALUES (390,10254,NULL,'client');
INSERT INTO `labels` VALUES (391,10255,NULL,'à-valider');
INSERT INTO `labels` VALUES (392,10257,NULL,'frontend');
INSERT INTO `labels` VALUES (393,10257,NULL,'régression');
INSERT INTO `labels` VALUES (394,10258,NULL,'dette-technique');
INSERT INTO `labels` VALUES (395,10258,NULL,'perf');
INSERT INTO `labels` VALUES (396,10258,10109,'rgpd');
INSERT INTO `labels` VALUES (397,10258,10109,'onboarding');
INSERT INTO `labels` VALUES (398,10262,10109,'rgpd');
INSERT INTO `labels` VALUES (399,10262,10109,'facturation');
INSERT INTO `labels` VALUES (400,10263,NULL,'sécurité');
INSERT INTO `labels` VALUES (401,10263,10109,'onboarding');
INSERT INTO `labels` VALUES (402,10263,10109,'rgpd');
INSERT INTO `labels` VALUES (403,10265,NULL,'ux');
INSERT INTO `labels` VALUES (404,10265,NULL,'backend');
INSERT INTO `labels` VALUES (405,10265,NULL,'perf');
INSERT INTO `labels` VALUES (406,10266,NULL,'mobile');
INSERT INTO `labels` VALUES (407,10266,NULL,'ux');
INSERT INTO `labels` VALUES (408,10266,NULL,'à-valider');
INSERT INTO `labels` VALUES (409,10267,NULL,'frontend');
INSERT INTO `labels` VALUES (410,10270,NULL,'backend');
INSERT INTO `labels` VALUES (411,10270,NULL,'perf');
INSERT INTO `labels` VALUES (412,10270,NULL,'client');
INSERT INTO `labels` VALUES (413,10272,10109,'facturation');
INSERT INTO `labels` VALUES (414,10273,NULL,'sécurité');
INSERT INTO `labels` VALUES (415,10273,NULL,'urgent');
INSERT INTO `labels` VALUES (416,10274,NULL,'mobile');
INSERT INTO `labels` VALUES (417,10274,NULL,'perf');
INSERT INTO `labels` VALUES (418,10274,NULL,'ux');
INSERT INTO `labels` VALUES (419,10276,NULL,'à-valider');
INSERT INTO `labels` VALUES (420,10276,NULL,'régression');
INSERT INTO `labels` VALUES (421,10276,NULL,'perf');
INSERT INTO `labels` VALUES (422,10277,NULL,'sécurité');
INSERT INTO `labels` VALUES (423,10277,NULL,'frontend');
INSERT INTO `labels` VALUES (424,10278,NULL,'régression');
INSERT INTO `labels` VALUES (425,10278,NULL,'perf');
INSERT INTO `labels` VALUES (426,10278,NULL,'backend');
INSERT INTO `labels` VALUES (427,10279,NULL,'ux');
INSERT INTO `labels` VALUES (428,10279,NULL,'perf');
INSERT INTO `labels` VALUES (429,10279,NULL,'client');
INSERT INTO `labels` VALUES (430,10282,NULL,'backend');
INSERT INTO `labels` VALUES (431,10283,NULL,'client');
INSERT INTO `labels` VALUES (432,10283,NULL,'frontend');
INSERT INTO `labels` VALUES (433,10285,NULL,'urgent');
INSERT INTO `labels` VALUES (434,10285,NULL,'backend');
INSERT INTO `labels` VALUES (435,10288,NULL,'client');
INSERT INTO `labels` VALUES (436,10292,10109,'facturation');
INSERT INTO `labels` VALUES (437,10293,10109,'accessibilité');
INSERT INTO `labels` VALUES (438,10293,10109,'onboarding');
INSERT INTO `labels` VALUES (439,10294,NULL,'ux');
INSERT INTO `labels` VALUES (440,10296,NULL,'régression');
INSERT INTO `labels` VALUES (441,10296,NULL,'frontend');
INSERT INTO `labels` VALUES (442,10296,NULL,'sécurité');
/*!40000 ALTER TABLE `labels` ENABLE KEYS */;
DROP TABLE IF EXISTS `link_types`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `link_types` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL,
  `inward` varchar(255) NOT NULL,
  `outward` varchar(255) NOT NULL,
  `position` int(11) NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=10004 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `link_types` DISABLE KEYS */;
INSERT INTO `link_types` VALUES (10000,'Blocks','is blocked by','blocks',1);
INSERT INTO `link_types` VALUES (10001,'Cloners','is cloned by','clones',2);
INSERT INTO `link_types` VALUES (10002,'Duplicate','is duplicated by','duplicates',3);
INSERT INTO `link_types` VALUES (10003,'Relates','relates to','relates to',4);
/*!40000 ALTER TABLE `link_types` ENABLE KEYS */;
DROP TABLE IF EXISTS `mail_outbox`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `mail_outbox` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `kind` varchar(20) NOT NULL,
  `user_id` bigint(20) DEFAULT NULL,
  `to_addr` varchar(320) NOT NULL,
  `subject` varchar(998) NOT NULL,
  `text_body` mediumtext NOT NULL,
  `html_body` mediumtext NOT NULL,
  `message_id` varchar(255) NOT NULL,
  `in_reply_to` varchar(255) DEFAULT NULL,
  `status` varchar(10) NOT NULL DEFAULT 'pending',
  `attempts` int(11) NOT NULL DEFAULT 0,
  `next_attempt_at` datetime(3) NOT NULL,
  `claim` char(32) DEFAULT NULL,
  `last_error` text DEFAULT NULL,
  `created_at` datetime(3) NOT NULL,
  `sent_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_mo_due` (`status`,`next_attempt_at`),
  KEY `ix_mo_claim` (`claim`),
  KEY `fk_mo_user` (`user_id`),
  CONSTRAINT `fk_mo_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `mail_outbox` DISABLE KEYS */;
/*!40000 ALTER TABLE `mail_outbox` ENABLE KEYS */;
DROP TABLE IF EXISTS `moved_issue_keys`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `moved_issue_keys` (
  `old_key` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  `issue_id` bigint(20) NOT NULL,
  PRIMARY KEY (`old_key`),
  KEY `fk_mik_issue` (`issue_id`),
  CONSTRAINT `fk_mik_issue` FOREIGN KEY (`issue_id`) REFERENCES `issues` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `moved_issue_keys` DISABLE KEYS */;
INSERT INTO `moved_issue_keys` VALUES ('OPS-5',10025);
/*!40000 ALTER TABLE `moved_issue_keys` ENABLE KEYS */;
DROP TABLE IF EXISTS `notification_batches`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `notification_batches` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `user_id` bigint(20) NOT NULL,
  `digest` tinyint(1) NOT NULL,
  `issue_id` bigint(20) NOT NULL,
  `first_at` datetime(3) NOT NULL,
  `send_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_nb` (`user_id`,`digest`,`issue_id`),
  KEY `ix_nb_send` (`send_at`),
  CONSTRAINT `fk_nb_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `notification_batches` DISABLE KEYS */;
/*!40000 ALTER TABLE `notification_batches` ENABLE KEYS */;
DROP TABLE IF EXISTS `notification_items`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `notification_items` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `batch_id` bigint(20) NOT NULL,
  `issue_id` bigint(20) NOT NULL,
  `data` mediumtext NOT NULL,
  `created_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_ni_batch` (`batch_id`),
  CONSTRAINT `fk_ni_batch` FOREIGN KEY (`batch_id`) REFERENCES `notification_batches` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `notification_items` DISABLE KEYS */;
/*!40000 ALTER TABLE `notification_items` ENABLE KEYS */;
DROP TABLE IF EXISTS `notification_prefs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `notification_prefs` (
  `user_id` bigint(20) NOT NULL,
  `delivery` varchar(10) NOT NULL DEFAULT 'immediate',
  `digest_hour` tinyint(4) NOT NULL DEFAULT 8,
  `own_changes` tinyint(1) NOT NULL DEFAULT 0,
  `disabled_events` varchar(255) NOT NULL DEFAULT '',
  `due_reminder` tinyint(1) NOT NULL DEFAULT 0,
  `reminder_hour` tinyint(4) NOT NULL DEFAULT 8,
  `reminder_sent` date DEFAULT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`user_id`),
  CONSTRAINT `fk_np_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `notification_prefs` DISABLE KEYS */;
/*!40000 ALTER TABLE `notification_prefs` ENABLE KEYS */;
DROP TABLE IF EXISTS `priorities`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `priorities` (
  `id` varchar(60) NOT NULL,
  `name` varchar(255) NOT NULL,
  `description` text DEFAULT NULL,
  `color` varchar(20) DEFAULT NULL,
  `position` int(11) NOT NULL DEFAULT 0,
  `is_default` tinyint(1) NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `priorities` DISABLE KEYS */;
INSERT INTO `priorities` VALUES ('1','Highest','Priorité Highest','#d04437',0,0);
INSERT INTO `priorities` VALUES ('2','High','Priorité High','#f15c75',1,0);
INSERT INTO `priorities` VALUES ('3','Medium','Priorité Medium','#f79232',2,0);
INSERT INTO `priorities` VALUES ('4','Low','Priorité Low','#707070',3,0);
INSERT INTO `priorities` VALUES ('5','Lowest','Priorité Lowest','#999999',4,0);
/*!40000 ALTER TABLE `priorities` ENABLE KEYS */;
DROP TABLE IF EXISTS `project_categories`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `project_categories` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL,
  `description` text DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=10002 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `project_categories` DISABLE KEYS */;
INSERT INTO `project_categories` VALUES (10000,'Produit','Projets liés au produit vendu aux clients');
INSERT INTO `project_categories` VALUES (10001,'Interne','Projets internes de l\'entreprise');
/*!40000 ALTER TABLE `project_categories` ENABLE KEYS */;
DROP TABLE IF EXISTS `project_issue_types`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `project_issue_types` (
  `project_id` bigint(20) NOT NULL,
  `issue_type_id` varchar(60) NOT NULL,
  `position` int(11) NOT NULL DEFAULT 0,
  PRIMARY KEY (`project_id`,`issue_type_id`),
  KEY `fk_pit_type` (`issue_type_id`),
  CONSTRAINT `fk_pit_project` FOREIGN KEY (`project_id`) REFERENCES `projects` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_pit_type` FOREIGN KEY (`issue_type_id`) REFERENCES `issue_types` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `project_issue_types` DISABLE KEYS */;
INSERT INTO `project_issue_types` VALUES (10000,'1',1);
INSERT INTO `project_issue_types` VALUES (10000,'10000',3);
INSERT INTO `project_issue_types` VALUES (10000,'10001',0);
INSERT INTO `project_issue_types` VALUES (10000,'3',2);
INSERT INTO `project_issue_types` VALUES (10000,'5',4);
INSERT INTO `project_issue_types` VALUES (10001,'1',1);
INSERT INTO `project_issue_types` VALUES (10001,'10000',3);
INSERT INTO `project_issue_types` VALUES (10001,'10001',0);
INSERT INTO `project_issue_types` VALUES (10001,'3',2);
INSERT INTO `project_issue_types` VALUES (10001,'5',4);
INSERT INTO `project_issue_types` VALUES (10002,'1',0);
INSERT INTO `project_issue_types` VALUES (10002,'10000',4);
INSERT INTO `project_issue_types` VALUES (10002,'10001',5);
INSERT INTO `project_issue_types` VALUES (10002,'3',1);
INSERT INTO `project_issue_types` VALUES (10002,'4',2);
INSERT INTO `project_issue_types` VALUES (10002,'5',3);
INSERT INTO `project_issue_types` VALUES (10003,'1',0);
INSERT INTO `project_issue_types` VALUES (10003,'10000',4);
INSERT INTO `project_issue_types` VALUES (10003,'10001',5);
INSERT INTO `project_issue_types` VALUES (10003,'3',1);
INSERT INTO `project_issue_types` VALUES (10003,'4',2);
INSERT INTO `project_issue_types` VALUES (10003,'5',3);
/*!40000 ALTER TABLE `project_issue_types` ENABLE KEYS */;
DROP TABLE IF EXISTS `project_keys`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `project_keys` (
  `pkey` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  `project_id` bigint(20) NOT NULL,
  PRIMARY KEY (`pkey`),
  KEY `fk_project_keys_project` (`project_id`),
  CONSTRAINT `fk_project_keys_project` FOREIGN KEY (`project_id`) REFERENCES `projects` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `project_keys` DISABLE KEYS */;
INSERT INTO `project_keys` VALUES ('SITE',10001);
/*!40000 ALTER TABLE `project_keys` ENABLE KEYS */;
DROP TABLE IF EXISTS `project_notification_recipients`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `project_notification_recipients` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `project_id` bigint(20) NOT NULL,
  `event` varchar(20) NOT NULL,
  `group_id` bigint(20) DEFAULT NULL,
  `role_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_pnr_project` (`project_id`),
  KEY `fk_pnr_group` (`group_id`),
  KEY `fk_pnr_role` (`role_id`),
  CONSTRAINT `fk_pnr_group` FOREIGN KEY (`group_id`) REFERENCES `user_groups` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_pnr_project` FOREIGN KEY (`project_id`) REFERENCES `projects` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_pnr_role` FOREIGN KEY (`role_id`) REFERENCES `project_roles` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `project_notification_recipients` DISABLE KEYS */;
/*!40000 ALTER TABLE `project_notification_recipients` ENABLE KEYS */;
DROP TABLE IF EXISTS `project_notifications`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `project_notifications` (
  `project_id` bigint(20) NOT NULL,
  `enabled` tinyint(1) NOT NULL DEFAULT 1,
  PRIMARY KEY (`project_id`),
  CONSTRAINT `fk_pn_project` FOREIGN KEY (`project_id`) REFERENCES `projects` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `project_notifications` DISABLE KEYS */;
/*!40000 ALTER TABLE `project_notifications` ENABLE KEYS */;
DROP TABLE IF EXISTS `project_permissions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `project_permissions` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `project_id` bigint(20) NOT NULL,
  `permission` varchar(40) NOT NULL,
  `holder` varchar(20) NOT NULL,
  `user_id` bigint(20) DEFAULT NULL,
  `group_id` bigint(20) DEFAULT NULL,
  `role_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_pp_project` (`project_id`,`permission`),
  KEY `fk_pp_user` (`user_id`),
  KEY `fk_pp_group` (`group_id`),
  KEY `fk_pp_role` (`role_id`),
  CONSTRAINT `fk_pp_group` FOREIGN KEY (`group_id`) REFERENCES `user_groups` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_pp_project` FOREIGN KEY (`project_id`) REFERENCES `projects` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_pp_role` FOREIGN KEY (`role_id`) REFERENCES `project_roles` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_pp_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=72 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `project_permissions` DISABLE KEYS */;
INSERT INTO `project_permissions` VALUES (1,10000,'browse','everyone',NULL,NULL,NULL);
INSERT INTO `project_permissions` VALUES (2,10000,'create','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (3,10000,'edit','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (4,10000,'comment','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (5,10000,'transition','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (6,10000,'attach','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (7,10000,'link','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (8,10000,'worklog','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (9,10000,'assignable','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (10,10000,'assign','role',NULL,NULL,10001);
INSERT INTO `project_permissions` VALUES (11,10000,'watchers','role',NULL,NULL,10001);
INSERT INTO `project_permissions` VALUES (12,10000,'security','role',NULL,NULL,10001);
INSERT INTO `project_permissions` VALUES (13,10000,'delete','role',NULL,NULL,10002);
INSERT INTO `project_permissions` VALUES (14,10000,'admin','role',NULL,NULL,10002);
INSERT INTO `project_permissions` VALUES (15,10000,'edit','reporter',NULL,NULL,NULL);
INSERT INTO `project_permissions` VALUES (16,10000,'transition','assignee',NULL,NULL,NULL);
INSERT INTO `project_permissions` VALUES (17,10000,'admin','lead',NULL,NULL,NULL);
INSERT INTO `project_permissions` VALUES (18,10000,'admin','user',2,NULL,NULL);
INSERT INTO `project_permissions` VALUES (19,10000,'link','group',NULL,3,NULL);
INSERT INTO `project_permissions` VALUES (20,10000,'moderate','role',NULL,NULL,10002);
INSERT INTO `project_permissions` VALUES (21,10001,'browse','everyone',NULL,NULL,NULL);
INSERT INTO `project_permissions` VALUES (22,10001,'create','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (23,10001,'edit','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (24,10001,'comment','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (25,10001,'transition','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (26,10001,'attach','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (27,10001,'link','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (28,10001,'worklog','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (29,10001,'assignable','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (30,10001,'assign','role',NULL,NULL,10001);
INSERT INTO `project_permissions` VALUES (31,10001,'watchers','role',NULL,NULL,10001);
INSERT INTO `project_permissions` VALUES (32,10001,'security','role',NULL,NULL,10001);
INSERT INTO `project_permissions` VALUES (33,10001,'delete','role',NULL,NULL,10002);
INSERT INTO `project_permissions` VALUES (34,10001,'admin','role',NULL,NULL,10002);
INSERT INTO `project_permissions` VALUES (35,10001,'edit','reporter',NULL,NULL,NULL);
INSERT INTO `project_permissions` VALUES (36,10001,'transition','assignee',NULL,NULL,NULL);
INSERT INTO `project_permissions` VALUES (37,10001,'admin','lead',NULL,NULL,NULL);
INSERT INTO `project_permissions` VALUES (38,10001,'admin','user',2,NULL,NULL);
INSERT INTO `project_permissions` VALUES (39,10001,'link','group',NULL,3,NULL);
INSERT INTO `project_permissions` VALUES (40,10001,'moderate','role',NULL,NULL,10002);
INSERT INTO `project_permissions` VALUES (41,10002,'browse','group',NULL,5,NULL);
INSERT INTO `project_permissions` VALUES (42,10002,'browse','role',NULL,NULL,10002);
INSERT INTO `project_permissions` VALUES (43,10002,'create','group',NULL,5,NULL);
INSERT INTO `project_permissions` VALUES (44,10002,'edit','group',NULL,5,NULL);
INSERT INTO `project_permissions` VALUES (45,10002,'comment','group',NULL,5,NULL);
INSERT INTO `project_permissions` VALUES (46,10002,'transition','group',NULL,5,NULL);
INSERT INTO `project_permissions` VALUES (47,10002,'assign','group',NULL,5,NULL);
INSERT INTO `project_permissions` VALUES (48,10002,'assignable','group',NULL,5,NULL);
INSERT INTO `project_permissions` VALUES (49,10002,'attach','group',NULL,5,NULL);
INSERT INTO `project_permissions` VALUES (50,10002,'admin','role',NULL,NULL,10002);
INSERT INTO `project_permissions` VALUES (51,10002,'delete','role',NULL,NULL,10002);
INSERT INTO `project_permissions` VALUES (52,10003,'browse','everyone',NULL,NULL,NULL);
INSERT INTO `project_permissions` VALUES (53,10003,'create','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (54,10003,'edit','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (55,10003,'comment','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (56,10003,'transition','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (57,10003,'attach','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (58,10003,'link','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (59,10003,'worklog','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (60,10003,'assignable','role',NULL,NULL,10000);
INSERT INTO `project_permissions` VALUES (61,10003,'assign','role',NULL,NULL,10001);
INSERT INTO `project_permissions` VALUES (62,10003,'watchers','role',NULL,NULL,10001);
INSERT INTO `project_permissions` VALUES (63,10003,'security','role',NULL,NULL,10001);
INSERT INTO `project_permissions` VALUES (64,10003,'delete','role',NULL,NULL,10002);
INSERT INTO `project_permissions` VALUES (65,10003,'admin','role',NULL,NULL,10002);
INSERT INTO `project_permissions` VALUES (66,10003,'edit','reporter',NULL,NULL,NULL);
INSERT INTO `project_permissions` VALUES (67,10003,'transition','assignee',NULL,NULL,NULL);
INSERT INTO `project_permissions` VALUES (68,10003,'admin','lead',NULL,NULL,NULL);
INSERT INTO `project_permissions` VALUES (69,10003,'admin','user',2,NULL,NULL);
INSERT INTO `project_permissions` VALUES (70,10003,'link','group',NULL,3,NULL);
INSERT INTO `project_permissions` VALUES (71,10003,'moderate','role',NULL,NULL,10002);
/*!40000 ALTER TABLE `project_permissions` ENABLE KEYS */;
DROP TABLE IF EXISTS `project_role_members`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `project_role_members` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `project_id` bigint(20) NOT NULL,
  `role_id` bigint(20) NOT NULL,
  `user_id` bigint(20) DEFAULT NULL,
  `group_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_prm_project` (`project_id`,`role_id`),
  KEY `fk_prm_role` (`role_id`),
  KEY `fk_prm_user` (`user_id`),
  KEY `fk_prm_group` (`group_id`),
  CONSTRAINT `fk_prm_group` FOREIGN KEY (`group_id`) REFERENCES `user_groups` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_prm_project` FOREIGN KEY (`project_id`) REFERENCES `projects` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_prm_role` FOREIGN KEY (`role_id`) REFERENCES `project_roles` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_prm_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=14 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `project_role_members` DISABLE KEYS */;
INSERT INTO `project_role_members` VALUES (1,10000,10000,NULL,2);
INSERT INTO `project_role_members` VALUES (2,10000,10001,NULL,3);
INSERT INTO `project_role_members` VALUES (3,10000,10001,9,NULL);
INSERT INTO `project_role_members` VALUES (4,10000,10002,1,NULL);
INSERT INTO `project_role_members` VALUES (5,10001,10000,NULL,2);
INSERT INTO `project_role_members` VALUES (6,10001,10001,NULL,3);
INSERT INTO `project_role_members` VALUES (7,10001,10001,9,NULL);
INSERT INTO `project_role_members` VALUES (8,10001,10002,1,NULL);
INSERT INTO `project_role_members` VALUES (9,10002,10000,NULL,5);
INSERT INTO `project_role_members` VALUES (10,10003,10000,NULL,2);
INSERT INTO `project_role_members` VALUES (11,10003,10001,NULL,3);
INSERT INTO `project_role_members` VALUES (12,10003,10001,9,NULL);
INSERT INTO `project_role_members` VALUES (13,10003,10002,1,NULL);
/*!40000 ALTER TABLE `project_role_members` ENABLE KEYS */;
DROP TABLE IF EXISTS `project_roles`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `project_roles` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL,
  `description` text DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=10003 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `project_roles` DISABLE KEYS */;
INSERT INTO `project_roles` VALUES (10000,'Users','Utilisateurs du projet');
INSERT INTO `project_roles` VALUES (10001,'Developers','Développeurs du projet');
INSERT INTO `project_roles` VALUES (10002,'Administrators','Administrateurs du projet');
/*!40000 ALTER TABLE `project_roles` ENABLE KEYS */;
DROP TABLE IF EXISTS `projects`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `projects` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `pkey` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  `name` varchar(255) NOT NULL,
  `description` text DEFAULT NULL,
  `lead_id` bigint(20) DEFAULT NULL,
  `category_id` bigint(20) DEFAULT NULL,
  `url` varchar(255) DEFAULT NULL,
  `kind` varchar(60) NOT NULL DEFAULT 'software',
  `default_assignee` enum('unassigned','lead') NOT NULL DEFAULT 'unassigned',
  `issue_counter` bigint(20) NOT NULL DEFAULT 0,
  `archived` tinyint(1) NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_projects_pkey` (`pkey`),
  KEY `fk_projects_lead` (`lead_id`),
  KEY `fk_projects_category` (`category_id`),
  CONSTRAINT `fk_projects_category` FOREIGN KEY (`category_id`) REFERENCES `project_categories` (`id`) ON DELETE SET NULL,
  CONSTRAINT `fk_projects_lead` FOREIGN KEY (`lead_id`) REFERENCES `users` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB AUTO_INCREMENT=10004 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `projects` DISABLE KEYS */;
INSERT INTO `projects` VALUES (10000,'DEMO','Démo','Projet « Démo »',2,10000,NULL,'software','unassigned',121,0);
INSERT INTO `projects` VALUES (10001,'WEB','Site web','Projet « Site web »',3,10000,NULL,'software','lead',90,0);
INSERT INTO `projects` VALUES (10002,'RH','Ressources humaines','Projet « Ressources humaines »',8,10001,NULL,'business','unassigned',30,0);
INSERT INTO `projects` VALUES (10003,'OPS','Exploitation','Projet « Exploitation »',7,10001,NULL,'business','lead',60,0);
/*!40000 ALTER TABLE `projects` ENABLE KEYS */;
DROP TABLE IF EXISTS `remote_links`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `remote_links` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `issue_id` bigint(20) NOT NULL,
  `url` text NOT NULL,
  `title` varchar(255) NOT NULL,
  `relationship` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_remote_links_issue` (`issue_id`),
  CONSTRAINT `fk_remote_links_issue` FOREIGN KEY (`issue_id`) REFERENCES `issues` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=10011 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `remote_links` DISABLE KEYS */;
INSERT INTO `remote_links` VALUES (10000,10282,'https://gitlab.exemple.fr/boutique/web/-/merge_requests/100','MR !100 — la gestion des rôles','mentionné dans');
INSERT INTO `remote_links` VALUES (10001,10017,'https://gitlab.exemple.fr/boutique/web/-/merge_requests/107','MR !107 — les traductions anglaises','mentionné dans');
INSERT INTO `remote_links` VALUES (10002,10089,'https://gitlab.exemple.fr/boutique/web/-/merge_requests/114','MR !114 — la gestion des rôles','mentionné dans');
INSERT INTO `remote_links` VALUES (10003,10142,'https://gitlab.exemple.fr/boutique/web/-/merge_requests/121','MR !121 — le système de notifications','mentionné dans');
INSERT INTO `remote_links` VALUES (10004,10215,'https://gitlab.exemple.fr/boutique/web/-/merge_requests/128','MR !128 — la configuration du cache','mentionné dans');
INSERT INTO `remote_links` VALUES (10005,10229,'https://gitlab.exemple.fr/boutique/web/-/merge_requests/135','MR !135 — la configuration du cache','mentionné dans');
INSERT INTO `remote_links` VALUES (10006,10244,'https://gitlab.exemple.fr/boutique/web/-/merge_requests/142','MR !142 — la configuration du cache','mentionné dans');
INSERT INTO `remote_links` VALUES (10007,10011,'https://gitlab.exemple.fr/boutique/web/-/merge_requests/149','MR !149 — les traductions anglaises','mentionné dans');
INSERT INTO `remote_links` VALUES (10008,10208,'https://gitlab.exemple.fr/boutique/web/-/merge_requests/156','MR !156 — les tests de bout en bout','mentionné dans');
INSERT INTO `remote_links` VALUES (10009,10072,'https://gitlab.exemple.fr/boutique/web/-/merge_requests/163','MR !163 — le parcours d\'inscription','mentionné dans');
INSERT INTO `remote_links` VALUES (10010,10215,'https://gitlab.exemple.fr/boutique/web/-/merge_requests/170','MR !170 — le parcours d\'inscription','mentionné dans');
/*!40000 ALTER TABLE `remote_links` ENABLE KEYS */;
DROP TABLE IF EXISTS `resolutions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `resolutions` (
  `id` varchar(60) NOT NULL,
  `name` varchar(255) NOT NULL,
  `description` text DEFAULT NULL,
  `position` int(11) NOT NULL DEFAULT 0,
  `is_default` tinyint(1) NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `resolutions` DISABLE KEYS */;
INSERT INTO `resolutions` VALUES ('1','Fixed','Résolution Fixed',0,1);
INSERT INTO `resolutions` VALUES ('10000','Done','Résolution Done',5,0);
INSERT INTO `resolutions` VALUES ('2','Won\'t Fix','Résolution Won\'t Fix',1,0);
INSERT INTO `resolutions` VALUES ('3','Duplicate','Résolution Duplicate',2,0);
INSERT INTO `resolutions` VALUES ('4','Incomplete','Résolution Incomplete',3,0);
INSERT INTO `resolutions` VALUES ('5','Cannot Reproduce','Résolution Cannot Reproduce',4,0);
/*!40000 ALTER TABLE `resolutions` ENABLE KEYS */;
DROP TABLE IF EXISTS `schema_migrations`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `schema_migrations` (
  `version` varchar(255) NOT NULL,
  `applied_at` datetime(3) NOT NULL,
  PRIMARY KEY (`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `schema_migrations` DISABLE KEYS */;
INSERT INTO `schema_migrations` VALUES ('0001_init','2026-10-01 18:00:00.000');
INSERT INTO `schema_migrations` VALUES ('0002_workflows','2026-10-01 18:00:00.000');
INSERT INTO `schema_migrations` VALUES ('0003_restrict_visibility','2026-10-01 18:00:00.000');
INSERT INTO `schema_migrations` VALUES ('0004_filters','2026-10-01 18:00:00.000');
INSERT INTO `schema_migrations` VALUES ('0005_boards','2026-10-01 18:00:00.000');
INSERT INTO `schema_migrations` VALUES ('0006_history_index','2026-10-01 18:00:00.000');
INSERT INTO `schema_migrations` VALUES ('0007_user_locale','2026-10-01 18:00:00.000');
INSERT INTO `schema_migrations` VALUES ('0008_notifications','2026-10-01 18:00:00.000');
INSERT INTO `schema_migrations` VALUES ('0009_api_tokens_audit','2026-10-01 18:00:00.000');
INSERT INTO `schema_migrations` VALUES ('0010_admin','2026-10-01 18:00:00.000');
INSERT INTO `schema_migrations` VALUES ('0011_default_resolution','2026-10-01 18:00:00.000');
INSERT INTO `schema_migrations` VALUES ('0012_account_tokens','2026-10-01 18:00:00.000');
/*!40000 ALTER TABLE `schema_migrations` ENABLE KEYS */;
DROP TABLE IF EXISTS `security_level_members`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `security_level_members` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `level_id` bigint(20) NOT NULL,
  `holder` varchar(20) NOT NULL,
  `user_id` bigint(20) DEFAULT NULL,
  `group_id` bigint(20) DEFAULT NULL,
  `role_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_slm_level` (`level_id`),
  KEY `fk_slm_user` (`user_id`),
  KEY `fk_slm_group` (`group_id`),
  KEY `fk_slm_role` (`role_id`),
  CONSTRAINT `fk_slm_group` FOREIGN KEY (`group_id`) REFERENCES `user_groups` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_slm_level` FOREIGN KEY (`level_id`) REFERENCES `security_levels` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_slm_role` FOREIGN KEY (`role_id`) REFERENCES `project_roles` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_slm_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `security_level_members` DISABLE KEYS */;
INSERT INTO `security_level_members` VALUES (1,1,'group',NULL,3,NULL);
INSERT INTO `security_level_members` VALUES (2,2,'reporter',NULL,NULL,NULL);
INSERT INTO `security_level_members` VALUES (3,2,'user',2,NULL,NULL);
INSERT INTO `security_level_members` VALUES (4,2,'role',NULL,NULL,10002);
/*!40000 ALTER TABLE `security_level_members` ENABLE KEYS */;
DROP TABLE IF EXISTS `security_levels`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `security_levels` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `project_id` bigint(20) NOT NULL,
  `name` varchar(255) NOT NULL,
  `description` text DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_sl_project` (`project_id`),
  CONSTRAINT `fk_sl_project` FOREIGN KEY (`project_id`) REFERENCES `projects` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `security_levels` DISABLE KEYS */;
INSERT INTO `security_levels` VALUES (1,10000,'Équipe','Visible de l\'équipe de développement');
INSERT INTO `security_levels` VALUES (2,10000,'Restreint','Visible du rapporteur et de la direction de projet');
/*!40000 ALTER TABLE `security_levels` ENABLE KEYS */;
DROP TABLE IF EXISTS `sessions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `sessions` (
  `token_hash` char(64) NOT NULL,
  `user_id` bigint(20) NOT NULL,
  `created_at` datetime(3) NOT NULL,
  `expires_at` datetime(3) NOT NULL,
  PRIMARY KEY (`token_hash`),
  KEY `ix_sessions_user` (`user_id`),
  CONSTRAINT `fk_sessions_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `sessions` DISABLE KEYS */;
/*!40000 ALTER TABLE `sessions` ENABLE KEYS */;
DROP TABLE IF EXISTS `settings`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `settings` (
  `name` varchar(191) NOT NULL,
  `value` mediumtext NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `settings` DISABLE KEYS */;
/*!40000 ALTER TABLE `settings` ENABLE KEYS */;
DROP TABLE IF EXISTS `sprint_issues`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `sprint_issues` (
  `sprint_id` bigint(20) NOT NULL,
  `issue_id` bigint(20) NOT NULL,
  PRIMARY KEY (`sprint_id`,`issue_id`),
  KEY `ix_si_issue` (`issue_id`),
  CONSTRAINT `fk_si_issue` FOREIGN KEY (`issue_id`) REFERENCES `issues` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_si_sprint` FOREIGN KEY (`sprint_id`) REFERENCES `sprints` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `sprint_issues` DISABLE KEYS */;
INSERT INTO `sprint_issues` VALUES (1,10018);
INSERT INTO `sprint_issues` VALUES (1,10061);
INSERT INTO `sprint_issues` VALUES (1,10238);
INSERT INTO `sprint_issues` VALUES (1,10241);
INSERT INTO `sprint_issues` VALUES (1,10289);
INSERT INTO `sprint_issues` VALUES (2,10029);
INSERT INTO `sprint_issues` VALUES (2,10089);
INSERT INTO `sprint_issues` VALUES (2,10163);
INSERT INTO `sprint_issues` VALUES (2,10297);
INSERT INTO `sprint_issues` VALUES (3,10072);
INSERT INTO `sprint_issues` VALUES (3,10074);
INSERT INTO `sprint_issues` VALUES (3,10090);
INSERT INTO `sprint_issues` VALUES (3,10134);
INSERT INTO `sprint_issues` VALUES (3,10186);
INSERT INTO `sprint_issues` VALUES (3,10212);
INSERT INTO `sprint_issues` VALUES (3,10240);
INSERT INTO `sprint_issues` VALUES (4,10049);
INSERT INTO `sprint_issues` VALUES (4,10068);
INSERT INTO `sprint_issues` VALUES (4,10146);
INSERT INTO `sprint_issues` VALUES (4,10190);
INSERT INTO `sprint_issues` VALUES (4,10200);
INSERT INTO `sprint_issues` VALUES (4,10284);
INSERT INTO `sprint_issues` VALUES (4,10295);
INSERT INTO `sprint_issues` VALUES (5,10001);
INSERT INTO `sprint_issues` VALUES (5,10010);
INSERT INTO `sprint_issues` VALUES (5,10017);
INSERT INTO `sprint_issues` VALUES (5,10045);
INSERT INTO `sprint_issues` VALUES (5,10068);
INSERT INTO `sprint_issues` VALUES (5,10109);
INSERT INTO `sprint_issues` VALUES (5,10129);
INSERT INTO `sprint_issues` VALUES (5,10138);
INSERT INTO `sprint_issues` VALUES (5,10146);
INSERT INTO `sprint_issues` VALUES (5,10190);
INSERT INTO `sprint_issues` VALUES (5,10195);
INSERT INTO `sprint_issues` VALUES (5,10279);
INSERT INTO `sprint_issues` VALUES (5,10280);
INSERT INTO `sprint_issues` VALUES (5,10284);
INSERT INTO `sprint_issues` VALUES (5,10287);
INSERT INTO `sprint_issues` VALUES (6,10019);
INSERT INTO `sprint_issues` VALUES (6,10032);
INSERT INTO `sprint_issues` VALUES (6,10112);
INSERT INTO `sprint_issues` VALUES (6,10152);
INSERT INTO `sprint_issues` VALUES (6,10174);
INSERT INTO `sprint_issues` VALUES (6,10181);
INSERT INTO `sprint_issues` VALUES (6,10184);
INSERT INTO `sprint_issues` VALUES (6,10217);
INSERT INTO `sprint_issues` VALUES (7,10013);
INSERT INTO `sprint_issues` VALUES (7,10078);
INSERT INTO `sprint_issues` VALUES (7,10098);
INSERT INTO `sprint_issues` VALUES (7,10180);
INSERT INTO `sprint_issues` VALUES (7,10247);
/*!40000 ALTER TABLE `sprint_issues` ENABLE KEYS */;
DROP TABLE IF EXISTS `sprints`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `sprints` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `board_id` bigint(20) DEFAULT NULL,
  `name` varchar(255) NOT NULL,
  `goal` text DEFAULT NULL,
  `state` enum('future','active','closed') NOT NULL,
  `start_at` datetime(3) DEFAULT NULL,
  `end_at` datetime(3) DEFAULT NULL,
  `completed_at` datetime(3) DEFAULT NULL,
  `position` bigint(20) NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `ix_sprints_board` (`board_id`,`state`),
  CONSTRAINT `fk_sprints_board` FOREIGN KEY (`board_id`) REFERENCES `boards` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB AUTO_INCREMENT=8 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `sprints` DISABLE KEYS */;
INSERT INTO `sprints` VALUES (1,1,'Sprint 1',NULL,'closed','2026-07-30 08:00:00.000','2026-08-13 16:00:00.000','2026-08-13 15:00:00.000',1);
INSERT INTO `sprints` VALUES (2,1,'Sprint 2',NULL,'closed','2026-08-13 08:00:00.000','2026-08-27 16:00:00.000','2026-08-27 15:00:00.000',2);
INSERT INTO `sprints` VALUES (3,1,'Sprint 3','Stabiliser le paiement','closed','2026-08-27 08:00:00.000','2026-09-10 16:00:00.000','2026-09-10 15:00:00.000',3);
INSERT INTO `sprints` VALUES (4,1,'Sprint 4',NULL,'closed','2026-09-10 08:00:00.000','2026-09-24 16:00:00.000','2026-09-24 15:00:00.000',4);
INSERT INTO `sprints` VALUES (5,1,'Sprint 5','Livrer l\'export des factures et fermer les anomalies bloquantes','active','2026-09-24 08:00:00.000','2026-10-08 16:00:00.000',NULL,5);
INSERT INTO `sprints` VALUES (6,1,'Sprint 6',NULL,'future',NULL,NULL,NULL,6);
INSERT INTO `sprints` VALUES (7,4,'RH — Octobre',NULL,'active','2026-09-28 08:00:00.000','2026-10-12 16:00:00.000',NULL,7);
/*!40000 ALTER TABLE `sprints` ENABLE KEYS */;
DROP TABLE IF EXISTS `statuses`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `statuses` (
  `id` varchar(60) NOT NULL,
  `name` varchar(255) NOT NULL,
  `description` text DEFAULT NULL,
  `category` enum('todo','doing','done') NOT NULL,
  `position` int(11) NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `statuses` DISABLE KEYS */;
INSERT INTO `statuses` VALUES ('1','Open','Statut « Open »','todo',0);
INSERT INTO `statuses` VALUES ('10000','To Do','Statut « To Do »','todo',5);
INSERT INTO `statuses` VALUES ('10001','Done','Statut « Done »','done',6);
INSERT INTO `statuses` VALUES ('10002','In Review','Statut « In Review »','doing',7);
INSERT INTO `statuses` VALUES ('3','In Progress','Statut « In Progress »','doing',1);
INSERT INTO `statuses` VALUES ('4','Reopened','Statut « Reopened »','todo',2);
INSERT INTO `statuses` VALUES ('5','Resolved','Statut « Resolved »','done',3);
INSERT INTO `statuses` VALUES ('6','Closed','Statut « Closed »','done',4);
/*!40000 ALTER TABLE `statuses` ENABLE KEYS */;
DROP TABLE IF EXISTS `user_groups`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `user_groups` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  `description` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_user_groups_name` (`name`)
) ENGINE=InnoDB AUTO_INCREMENT=8 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `user_groups` DISABLE KEYS */;
INSERT INTO `user_groups` VALUES (1,'jira-administrators','Administrateurs de Jira');
INSERT INTO `user_groups` VALUES (2,'jira-software-users','Utilisateurs de Jira Software');
INSERT INTO `user_groups` VALUES (3,'developpeurs','Équipe de développement');
INSERT INTO `user_groups` VALUES (4,'equipe-mobile','Équipe mobile');
INSERT INTO `user_groups` VALUES (5,'rh','Ressources humaines');
INSERT INTO `user_groups` VALUES (6,'direction','Direction');
INSERT INTO `user_groups` VALUES (7,'ldap-devs','Développeurs (annuaire LDAP)');
/*!40000 ALTER TABLE `user_groups` ENABLE KEYS */;
DROP TABLE IF EXISTS `users`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `users` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `username` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  `user_key` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin DEFAULT NULL,
  `display_name` varchar(255) NOT NULL,
  `email` varchar(255) NOT NULL DEFAULT '',
  `password_hash` varchar(255) DEFAULT NULL,
  `active` tinyint(1) NOT NULL DEFAULT 1,
  `is_admin` tinyint(1) NOT NULL DEFAULT 0,
  `created_at` datetime(3) NOT NULL DEFAULT current_timestamp(3),
  `locale` varchar(10) DEFAULT NULL,
  `last_login_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_users_username` (`username`),
  UNIQUE KEY `uq_users_user_key` (`user_key`)
) ENGINE=InnoDB AUTO_INCREMENT=25 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `users` DISABLE KEYS */;
INSERT INTO `users` VALUES (1,'admin','admin','Administrateur Jira','admin@exemple.fr','{PKCS5S2}MEAB9soi6lXWxnAi1ZpQwI1Bf+9UqaV7kvMi6emYcDosZpNrngOkUxMQIlyX+0pj',1,1,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (2,'marie.durand','JIRAUSER10000','Marie Durand','marie.durand@exemple.fr','{PKCS5S2}1xI6CHqVLCEfl7Duq/h22Z/yitXzJWGl450MB3HSU/Wcl0fct2e13UWwAUO2wJ7a',1,1,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (3,'paul.martin','paul.martin','Paul Martin','paul.martin@exemple.fr','{PKCS5S2}WwhSmXA0CGrztvGuxebc4QsW7R8rB7aMJeJg1asX8/oi5L2A3NG6OQABKMpzOzEt',1,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (4,'jeanne.dupont','jdoe','Jeanne Dupont','jeanne.dupont@exemple.fr','{PKCS5S2}v6mZ9DwXFyhV8m+VCB4A3EzU3iBT/x8DBOKfiZgOL/jIUYzodF9/GJuZuQJhTCNO',1,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (5,'lucas.bernard','JIRAUSER10001','Lucas Bernard','lucas.bernard@exemple.fr','{PKCS5S2}z2N+TUT/qyWiWp2ojK65e4O9Qcq6Q5KVtwmY30uz7x3bFafwatIzUHwOKtR1xKIi',1,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (6,'camille.petit','JIRAUSER10002','Camille Petit','camille.petit@exemple.fr','{PKCS5S2}6r9hQ81LQT0rbKPhh2aHoLSI9lMiEQWNkGXTvLkFiT61vhI4HGpxb0KWJ1H8AOR9',1,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (7,'hugo.robert','JIRAUSER10003','Hugo Robert','hugo.robert@exemple.fr','{PKCS5S2}6UQhrlSm26oCeJrPTKv0rnl+ueVw+5xoR9vS4/zNnh0Ic38qIzQ3sgT4Q6gGUtDj',1,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (8,'lea.richard','JIRAUSER10004','Léa Richard','lea.richard@exemple.fr','{PKCS5S2}6I7TEdI3lg3+88ddVNYh35rC9i5FDEvBmR3cHFySG6TsAX0D0y3av5udC8ScxvQ4',1,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (9,'thomas.moreau','JIRAUSER10005','Thomas Moreau','thomas.moreau@exemple.fr','{PKCS5S2}29D1E6H00sm5djR3IxKaSqEbDV3/GdtTxDCoWsQpQwVC4n0FuNUq3ePyXRWYEaEg',1,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (10,'chloe.simon','JIRAUSER10006','Chloé Simon','chloe.simon@exemple.fr','{PKCS5S2}ahwnq6vPfFoaRWYvoz4wM2oOcrJQGckMdJnuszVOw1lQgaAJo5SPbpneMRf9j8ft',1,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (11,'nicolas.laurent','JIRAUSER10007','Nicolas Laurent','nicolas.laurent@exemple.fr','{PKCS5S2}/w9gPWT/wkk2Shq06LxRqdv/cfwI7AxybaZIuDXvCyFGIt1sYJIU+qu66dARZ0XV',1,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (12,'manon.michel','JIRAUSER10008','Manon Michel','manon.michel@exemple.fr','{PKCS5S2}YWHtqgs0QOE+9n/+0Ppj1Xchkrt6N7kMR/lB6mnIRZ4010s6+t2BkzFE8iGX7KBf',1,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (13,'antoine.lefebvre','JIRAUSER10009','Antoine Lefebvre','antoine.lefebvre@exemple.fr','{PKCS5S2}MX6rG8c2yKcqP4EgUmN/axEA60AnAf03J64YJmgxrVKnMtZ3r3vgpZV4qjo0umRG',1,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (14,'sarah.leroy','JIRAUSER10010','Sarah Leroy','sarah.leroy@exemple.fr','{PKCS5S2}Gql35gzIbjAzfSBj+JqLT09XLeuD5XSwIsxOkLKS89J/TZRoYOkQkKHre6ANI1/2',1,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (15,'julien.roux','JIRAUSER10011','Julien Roux','julien.roux@exemple.fr','{PKCS5S2}sSEgMNRLVb9NYwGxD70KjpscpugvkwKck4tqVxf6jv1d/AiNDunkWGO0iOV7IIOz',1,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (16,'ines.david','JIRAUSER10012','Inès David','ines.david@exemple.fr','{PKCS5S2}p9ZHfYOA0EvBGcRJbQQyzGdkrUZPNTMno9LVTRSL8ftknYDylA0aK/nIWkTuiER1',1,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (17,'maxime.bertrand','JIRAUSER10013','Maxime Bertrand','maxime.bertrand@exemple.fr','{PKCS5S2}THLXZ8im4C9PXEUPx/c9HjtiszGsVRwze9yFkVNh72ttctMBJeePMG5Eatk1+IrW',1,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (18,'emma.morel','JIRAUSER10014','Emma Morel','emma.morel@exemple.fr','{PKCS5S2}jlKB2JfAa8eQ+5rl9gHPteeEbuk6U0pKPxe95oBeaUiVEHuP1zbR2pSijyWgqd8e',0,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (19,'louis.fournier','JIRAUSER10015','Louis Fournier','louis.fournier@exemple.fr','{PKCS5S2}phU4eqwYkpFgaMv1q2euaI5VYvJNyPZKx2hhAETsNYnehjencvKODSei9E8kev2x',1,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (20,'alice.girard','JIRAUSER10016','Alice Girard','alice.girard@exemple.fr','{PKCS5S2}CEWY1Fj6oLEnaUogvH2cFZVFCsFk1Onvfw5MgP+eu4+BwQswu1Hm58Qssadm4rE3',1,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (21,'karim.benali','JIRAUSER10017','Karim Benali','karim.benali@exemple.fr',NULL,0,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (22,'sophie.lambert','JIRAUSER10018','Sophie Lambert','sophie.lambert@exemple.fr',NULL,0,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (23,'yann.lecomte','JIRAUSER10019','Yann Lecomte','yann.lecomte@exemple.fr',NULL,0,0,'2026-10-01 18:00:00.000',NULL,NULL);
INSERT INTO `users` VALUES (24,'ancien.collegue','JIRAUSER10099','ancien.collegue','',NULL,0,0,'2026-10-01 18:00:00.000',NULL,NULL);
/*!40000 ALTER TABLE `users` ENABLE KEYS */;
DROP TABLE IF EXISTS `versions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `versions` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `project_id` bigint(20) NOT NULL,
  `name` varchar(255) NOT NULL,
  `description` text DEFAULT NULL,
  `position` int(11) NOT NULL DEFAULT 0,
  `released` tinyint(1) NOT NULL DEFAULT 0,
  `archived` tinyint(1) NOT NULL DEFAULT 0,
  `start_date` date DEFAULT NULL,
  `release_date` date DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_versions_project` (`project_id`),
  CONSTRAINT `fk_versions_project` FOREIGN KEY (`project_id`) REFERENCES `projects` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=10007 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `versions` DISABLE KEYS */;
INSERT INTO `versions` VALUES (10000,10000,'1.0','Version 1.0',1,1,1,'2025-08-27','2025-10-26');
INSERT INTO `versions` VALUES (10001,10000,'1.1','Version 1.1',2,1,0,'2025-10-26','2026-01-24');
INSERT INTO `versions` VALUES (10002,10000,'2.0','Version 2.0',3,0,0,'2026-01-24','2026-10-21');
INSERT INTO `versions` VALUES (10003,10000,'2.1','Version 2.1',4,0,0,'2026-06-23','2026-12-20');
INSERT INTO `versions` VALUES (10004,10001,'2025.1','Version 2025.1',1,1,0,'2025-08-27','2025-12-25');
INSERT INTO `versions` VALUES (10005,10001,'2025.2','Version 2025.2',2,1,0,'2025-12-25','2026-05-14');
INSERT INTO `versions` VALUES (10006,10001,'2026.1','Version 2026.1',3,0,0,'2026-05-14','2026-10-31');
/*!40000 ALTER TABLE `versions` ENABLE KEYS */;
DROP TABLE IF EXISTS `votes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `votes` (
  `issue_id` bigint(20) NOT NULL,
  `user_id` bigint(20) NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`issue_id`,`user_id`),
  KEY `ix_votes_user` (`user_id`),
  CONSTRAINT `fk_votes_issue` FOREIGN KEY (`issue_id`) REFERENCES `issues` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_votes_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `votes` DISABLE KEYS */;
INSERT INTO `votes` VALUES (10000,20,'2025-09-15 10:04:36.000');
INSERT INTO `votes` VALUES (10012,15,'2025-10-21 08:24:50.000');
INSERT INTO `votes` VALUES (10012,22,'2025-10-18 06:22:02.000');
INSERT INTO `votes` VALUES (10014,9,'2025-10-16 09:24:36.000');
INSERT INTO `votes` VALUES (10014,13,'2025-10-23 09:49:37.000');
INSERT INTO `votes` VALUES (10014,22,'2025-10-23 08:30:30.000');
INSERT INTO `votes` VALUES (10026,19,'2025-11-11 07:27:28.000');
INSERT INTO `votes` VALUES (10039,16,'2025-11-06 18:56:38.000');
INSERT INTO `votes` VALUES (10041,14,'2025-11-21 08:04:25.000');
INSERT INTO `votes` VALUES (10042,23,'2025-12-10 08:52:55.000');
INSERT INTO `votes` VALUES (10043,20,'2025-11-27 10:00:07.000');
INSERT INTO `votes` VALUES (10045,7,'2025-12-11 07:45:00.000');
INSERT INTO `votes` VALUES (10045,20,'2025-11-26 18:13:58.000');
INSERT INTO `votes` VALUES (10045,22,'2025-11-21 10:47:08.000');
INSERT INTO `votes` VALUES (10058,10,'2025-12-04 09:27:49.000');
INSERT INTO `votes` VALUES (10071,23,'2025-12-23 15:39:08.000');
INSERT INTO `votes` VALUES (10077,6,'2026-01-04 15:35:18.000');
INSERT INTO `votes` VALUES (10077,7,'2026-01-03 07:59:26.000');
INSERT INTO `votes` VALUES (10077,17,'2025-12-30 15:27:11.000');
INSERT INTO `votes` VALUES (10078,1,'2025-12-18 15:58:34.000');
INSERT INTO `votes` VALUES (10078,12,'2026-01-07 17:15:39.000');
INSERT INTO `votes` VALUES (10078,23,'2026-01-07 16:02:53.000');
INSERT INTO `votes` VALUES (10095,12,'2026-02-15 11:57:50.000');
INSERT INTO `votes` VALUES (10095,19,'2026-02-11 07:32:17.000');
INSERT INTO `votes` VALUES (10095,22,'2026-02-22 14:06:37.000');
INSERT INTO `votes` VALUES (10111,13,'2026-02-18 07:41:08.000');
INSERT INTO `votes` VALUES (10111,20,'2026-03-10 16:39:50.000');
INSERT INTO `votes` VALUES (10111,23,'2026-02-18 07:10:46.000');
INSERT INTO `votes` VALUES (10117,1,'2026-03-08 14:40:19.000');
INSERT INTO `votes` VALUES (10117,21,'2026-03-01 09:46:19.000');
INSERT INTO `votes` VALUES (10122,11,'2026-03-06 17:59:41.000');
INSERT INTO `votes` VALUES (10122,19,'2026-03-14 14:21:59.000');
INSERT INTO `votes` VALUES (10123,10,'2026-03-16 17:37:14.000');
INSERT INTO `votes` VALUES (10123,16,'2026-03-19 11:50:45.000');
INSERT INTO `votes` VALUES (10126,7,'2026-03-16 18:03:37.000');
INSERT INTO `votes` VALUES (10126,14,'2026-03-14 09:14:29.000');
INSERT INTO `votes` VALUES (10126,23,'2026-03-26 13:18:52.000');
INSERT INTO `votes` VALUES (10131,5,'2026-03-26 09:47:46.000');
INSERT INTO `votes` VALUES (10131,19,'2026-03-10 10:03:30.000');
INSERT INTO `votes` VALUES (10145,2,'2026-04-07 12:39:49.000');
INSERT INTO `votes` VALUES (10145,6,'2026-04-07 13:26:15.000');
INSERT INTO `votes` VALUES (10145,23,'2026-04-21 07:58:39.000');
INSERT INTO `votes` VALUES (10158,23,'2026-05-13 10:05:21.000');
INSERT INTO `votes` VALUES (10186,9,'2026-06-10 14:16:22.000');
INSERT INTO `votes` VALUES (10186,22,'2026-05-19 12:23:38.000');
INSERT INTO `votes` VALUES (10190,6,'2026-06-04 07:57:53.000');
INSERT INTO `votes` VALUES (10190,14,'2026-06-10 14:59:59.000');
INSERT INTO `votes` VALUES (10198,2,'2026-06-19 16:41:37.000');
INSERT INTO `votes` VALUES (10198,9,'2026-06-22 14:45:41.000');
INSERT INTO `votes` VALUES (10198,17,'2026-06-22 14:11:19.000');
INSERT INTO `votes` VALUES (10199,8,'2026-05-31 16:44:32.000');
INSERT INTO `votes` VALUES (10199,14,'2026-06-19 13:44:43.000');
INSERT INTO `votes` VALUES (10203,5,'2026-06-08 11:00:24.000');
INSERT INTO `votes` VALUES (10203,22,'2026-06-22 11:58:08.000');
INSERT INTO `votes` VALUES (10216,14,'2026-06-15 08:37:09.000');
INSERT INTO `votes` VALUES (10216,19,'2026-06-18 12:25:59.000');
INSERT INTO `votes` VALUES (10258,5,'2026-08-29 15:16:15.000');
INSERT INTO `votes` VALUES (10261,20,'2026-08-18 09:19:20.000');
INSERT INTO `votes` VALUES (10264,4,'2026-09-03 13:09:32.000');
INSERT INTO `votes` VALUES (10264,10,'2026-09-11 06:01:16.000');
/*!40000 ALTER TABLE `votes` ENABLE KEYS */;
DROP TABLE IF EXISTS `watchers`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `watchers` (
  `issue_id` bigint(20) NOT NULL,
  `user_id` bigint(20) NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`issue_id`,`user_id`),
  KEY `ix_watchers_user` (`user_id`),
  CONSTRAINT `fk_watchers_issue` FOREIGN KEY (`issue_id`) REFERENCES `issues` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_watchers_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `watchers` DISABLE KEYS */;
INSERT INTO `watchers` VALUES (10000,6,'2025-09-02 12:41:51.000');
INSERT INTO `watchers` VALUES (10000,16,'2025-08-29 17:52:32.000');
INSERT INTO `watchers` VALUES (10001,5,'2025-09-04 08:44:04.000');
INSERT INTO `watchers` VALUES (10001,10,'2025-09-06 08:28:55.000');
INSERT INTO `watchers` VALUES (10001,16,'2025-09-03 09:56:26.000');
INSERT INTO `watchers` VALUES (10002,2,'2025-09-06 09:15:15.000');
INSERT INTO `watchers` VALUES (10002,9,'2025-09-03 12:44:05.000');
INSERT INTO `watchers` VALUES (10002,13,'2025-09-01 16:05:57.000');
INSERT INTO `watchers` VALUES (10002,23,'2025-09-06 08:25:28.000');
INSERT INTO `watchers` VALUES (10003,15,'2025-09-12 07:27:55.000');
INSERT INTO `watchers` VALUES (10003,21,'2025-09-13 10:04:23.000');
INSERT INTO `watchers` VALUES (10004,5,'2025-09-12 09:22:18.000');
INSERT INTO `watchers` VALUES (10004,11,'2025-09-13 17:27:02.000');
INSERT INTO `watchers` VALUES (10004,17,'2025-09-15 11:47:09.000');
INSERT INTO `watchers` VALUES (10005,2,'2025-09-15 12:17:07.000');
INSERT INTO `watchers` VALUES (10005,10,'2025-09-15 15:21:37.000');
INSERT INTO `watchers` VALUES (10006,9,'2025-09-16 16:12:18.000');
INSERT INTO `watchers` VALUES (10006,19,'2025-09-16 17:34:30.000');
INSERT INTO `watchers` VALUES (10006,23,'2025-09-16 13:54:51.000');
INSERT INTO `watchers` VALUES (10007,7,'2025-09-20 14:25:22.000');
INSERT INTO `watchers` VALUES (10007,9,'2025-09-16 09:29:24.000');
INSERT INTO `watchers` VALUES (10007,17,'2025-09-16 10:11:48.000');
INSERT INTO `watchers` VALUES (10008,5,'2025-09-20 10:52:22.000');
INSERT INTO `watchers` VALUES (10008,17,'2025-09-22 09:31:16.000');
INSERT INTO `watchers` VALUES (10009,7,'2025-09-25 14:32:46.000');
INSERT INTO `watchers` VALUES (10010,4,'2025-09-22 15:52:19.000');
INSERT INTO `watchers` VALUES (10010,15,'2025-09-21 11:02:07.000');
INSERT INTO `watchers` VALUES (10010,20,'2025-09-23 17:32:46.000');
INSERT INTO `watchers` VALUES (10011,3,'2025-09-23 08:56:27.000');
INSERT INTO `watchers` VALUES (10011,4,'2025-09-21 17:19:57.000');
INSERT INTO `watchers` VALUES (10011,20,'2025-09-22 09:27:18.000');
INSERT INTO `watchers` VALUES (10011,21,'2025-09-25 17:26:45.000');
INSERT INTO `watchers` VALUES (10012,6,'2025-09-23 12:16:08.000');
INSERT INTO `watchers` VALUES (10012,13,'2025-09-23 14:54:13.000');
INSERT INTO `watchers` VALUES (10012,18,'2025-09-24 06:06:05.000');
INSERT INTO `watchers` VALUES (10013,1,'2025-09-26 08:46:12.000');
INSERT INTO `watchers` VALUES (10013,10,'2025-10-01 16:59:10.000');
INSERT INTO `watchers` VALUES (10013,15,'2025-09-29 13:36:21.000');
INSERT INTO `watchers` VALUES (10013,21,'2025-09-26 06:42:16.000');
INSERT INTO `watchers` VALUES (10014,14,'2025-09-27 17:05:20.000');
INSERT INTO `watchers` VALUES (10015,7,'2025-09-29 09:29:30.000');
INSERT INTO `watchers` VALUES (10015,11,'2025-10-03 06:50:38.000');
INSERT INTO `watchers` VALUES (10015,20,'2025-09-28 16:21:36.000');
INSERT INTO `watchers` VALUES (10016,19,'2025-10-02 16:25:31.000');
INSERT INTO `watchers` VALUES (10016,23,'2025-10-04 15:37:35.000');
INSERT INTO `watchers` VALUES (10017,6,'2025-10-03 13:40:18.000');
INSERT INTO `watchers` VALUES (10017,15,'2025-10-05 08:55:09.000');
INSERT INTO `watchers` VALUES (10018,3,'2025-10-07 13:06:46.000');
INSERT INTO `watchers` VALUES (10018,4,'2025-10-07 14:24:44.000');
INSERT INTO `watchers` VALUES (10018,6,'2025-10-09 12:16:19.000');
INSERT INTO `watchers` VALUES (10019,5,'2025-10-06 15:19:54.000');
INSERT INTO `watchers` VALUES (10019,17,'2025-10-09 13:52:55.000');
INSERT INTO `watchers` VALUES (10020,7,'2025-10-15 10:36:46.000');
INSERT INTO `watchers` VALUES (10020,8,'2025-10-15 15:20:24.000');
INSERT INTO `watchers` VALUES (10020,19,'2025-10-18 12:54:15.000');
INSERT INTO `watchers` VALUES (10021,12,'2025-10-21 11:12:04.000');
INSERT INTO `watchers` VALUES (10021,14,'2025-10-18 10:05:12.000');
INSERT INTO `watchers` VALUES (10022,9,'2025-10-19 13:34:49.000');
INSERT INTO `watchers` VALUES (10022,17,'2025-10-19 15:50:02.000');
INSERT INTO `watchers` VALUES (10023,9,'2025-10-17 17:03:52.000');
INSERT INTO `watchers` VALUES (10023,10,'2025-10-20 10:05:31.000');
INSERT INTO `watchers` VALUES (10024,2,'2025-10-22 16:12:32.000');
INSERT INTO `watchers` VALUES (10024,13,'2025-10-18 16:01:40.000');
INSERT INTO `watchers` VALUES (10024,22,'2025-10-18 16:08:35.000');
INSERT INTO `watchers` VALUES (10025,5,'2025-10-21 15:09:41.000');
INSERT INTO `watchers` VALUES (10025,17,'2025-10-20 09:50:03.000');
INSERT INTO `watchers` VALUES (10026,3,'2025-10-23 17:50:02.000');
INSERT INTO `watchers` VALUES (10026,13,'2025-10-20 13:39:15.000');
INSERT INTO `watchers` VALUES (10026,15,'2025-10-21 08:08:42.000');
INSERT INTO `watchers` VALUES (10027,9,'2025-10-24 13:40:44.000');
INSERT INTO `watchers` VALUES (10027,18,'2025-10-22 07:42:41.000');
INSERT INTO `watchers` VALUES (10028,8,'2025-10-22 09:38:05.000');
INSERT INTO `watchers` VALUES (10028,10,'2025-10-21 11:10:13.000');
INSERT INTO `watchers` VALUES (10029,6,'2025-10-24 15:17:11.000');
INSERT INTO `watchers` VALUES (10029,11,'2025-10-21 16:36:14.000');
INSERT INTO `watchers` VALUES (10030,2,'2025-10-25 08:20:57.000');
INSERT INTO `watchers` VALUES (10030,11,'2025-10-25 12:19:11.000');
INSERT INTO `watchers` VALUES (10030,12,'2025-10-23 17:25:34.000');
INSERT INTO `watchers` VALUES (10031,5,'2025-10-28 10:25:40.000');
INSERT INTO `watchers` VALUES (10031,11,'2025-10-27 16:16:55.000');
INSERT INTO `watchers` VALUES (10031,17,'2025-10-23 15:38:47.000');
INSERT INTO `watchers` VALUES (10032,6,'2025-10-26 09:01:37.000');
INSERT INTO `watchers` VALUES (10032,11,'2025-10-25 15:55:54.000');
INSERT INTO `watchers` VALUES (10032,16,'2025-10-24 17:53:18.000');
INSERT INTO `watchers` VALUES (10033,3,'2025-10-27 08:43:16.000');
INSERT INTO `watchers` VALUES (10033,8,'2025-10-26 15:14:59.000');
INSERT INTO `watchers` VALUES (10033,11,'2025-10-27 14:37:28.000');
INSERT INTO `watchers` VALUES (10033,13,'2025-10-26 14:07:05.000');
INSERT INTO `watchers` VALUES (10034,13,'2025-11-04 12:32:38.000');
INSERT INTO `watchers` VALUES (10034,16,'2025-11-03 12:57:36.000');
INSERT INTO `watchers` VALUES (10035,3,'2025-11-02 18:02:36.000');
INSERT INTO `watchers` VALUES (10035,5,'2025-11-02 16:02:52.000');
INSERT INTO `watchers` VALUES (10035,8,'2025-11-03 11:38:22.000');
INSERT INTO `watchers` VALUES (10035,16,'2025-11-05 08:36:05.000');
INSERT INTO `watchers` VALUES (10036,3,'2025-11-06 17:14:20.000');
INSERT INTO `watchers` VALUES (10036,11,'2025-11-06 17:43:10.000');
INSERT INTO `watchers` VALUES (10036,15,'2025-11-01 14:57:57.000');
INSERT INTO `watchers` VALUES (10036,18,'2025-11-05 10:37:22.000');
INSERT INTO `watchers` VALUES (10037,2,'2025-11-08 12:11:25.000');
INSERT INTO `watchers` VALUES (10037,5,'2025-11-05 12:51:17.000');
INSERT INTO `watchers` VALUES (10037,9,'2025-11-06 07:05:42.000');
INSERT INTO `watchers` VALUES (10038,1,'2025-11-10 14:56:54.000');
INSERT INTO `watchers` VALUES (10038,3,'2025-11-07 08:22:13.000');
INSERT INTO `watchers` VALUES (10038,12,'2025-11-11 12:28:05.000');
INSERT INTO `watchers` VALUES (10039,1,'2025-11-11 08:14:21.000');
INSERT INTO `watchers` VALUES (10039,4,'2025-11-07 13:38:53.000');
INSERT INTO `watchers` VALUES (10040,9,'2025-11-09 18:15:01.000');
INSERT INTO `watchers` VALUES (10041,3,'2025-11-12 16:26:11.000');
INSERT INTO `watchers` VALUES (10041,4,'2025-11-09 08:37:54.000');
INSERT INTO `watchers` VALUES (10041,13,'2025-11-10 16:09:38.000');
INSERT INTO `watchers` VALUES (10041,18,'2025-11-09 18:49:45.000');
INSERT INTO `watchers` VALUES (10042,4,'2025-11-11 07:12:18.000');
INSERT INTO `watchers` VALUES (10042,12,'2025-11-11 17:02:46.000');
INSERT INTO `watchers` VALUES (10042,13,'2025-11-13 09:57:46.000');
INSERT INTO `watchers` VALUES (10042,16,'2025-11-14 11:47:05.000');
INSERT INTO `watchers` VALUES (10043,7,'2025-11-15 12:08:11.000');
INSERT INTO `watchers` VALUES (10043,9,'2025-11-16 18:58:36.000');
INSERT INTO `watchers` VALUES (10043,19,'2025-11-14 10:59:19.000');
INSERT INTO `watchers` VALUES (10044,4,'2025-11-15 08:06:49.000');
INSERT INTO `watchers` VALUES (10044,10,'2025-11-18 12:47:49.000');
INSERT INTO `watchers` VALUES (10044,17,'2025-11-16 12:22:07.000');
INSERT INTO `watchers` VALUES (10045,9,'2025-11-18 17:09:50.000');
INSERT INTO `watchers` VALUES (10045,13,'2025-11-17 08:51:38.000');
INSERT INTO `watchers` VALUES (10045,23,'2025-11-16 12:35:37.000');
INSERT INTO `watchers` VALUES (10046,5,'2025-11-18 17:17:44.000');
INSERT INTO `watchers` VALUES (10046,7,'2025-11-21 10:05:32.000');
INSERT INTO `watchers` VALUES (10047,9,'2025-11-19 07:18:41.000');
INSERT INTO `watchers` VALUES (10047,22,'2025-11-21 16:59:38.000');
INSERT INTO `watchers` VALUES (10048,15,'2025-11-17 15:35:20.000');
INSERT INTO `watchers` VALUES (10048,17,'2025-11-21 16:55:21.000');
INSERT INTO `watchers` VALUES (10048,22,'2025-11-18 17:21:34.000');
INSERT INTO `watchers` VALUES (10049,4,'2025-11-20 13:41:22.000');
INSERT INTO `watchers` VALUES (10049,9,'2025-11-22 11:21:32.000');
INSERT INTO `watchers` VALUES (10049,21,'2025-11-20 14:29:30.000');
INSERT INTO `watchers` VALUES (10050,14,'2025-11-23 16:17:59.000');
INSERT INTO `watchers` VALUES (10050,20,'2025-11-21 16:42:20.000');
INSERT INTO `watchers` VALUES (10051,2,'2025-11-19 18:55:03.000');
INSERT INTO `watchers` VALUES (10051,8,'2025-11-23 15:50:52.000');
INSERT INTO `watchers` VALUES (10051,15,'2025-11-22 07:12:02.000');
INSERT INTO `watchers` VALUES (10052,1,'2025-11-25 14:46:29.000');
INSERT INTO `watchers` VALUES (10052,7,'2025-11-20 14:37:41.000');
INSERT INTO `watchers` VALUES (10052,11,'2025-11-24 15:33:24.000');
INSERT INTO `watchers` VALUES (10052,20,'2025-11-23 12:28:13.000');
INSERT INTO `watchers` VALUES (10053,9,'2025-11-25 18:58:13.000');
INSERT INTO `watchers` VALUES (10053,11,'2025-11-26 11:54:44.000');
INSERT INTO `watchers` VALUES (10054,5,'2025-11-25 17:14:25.000');
INSERT INTO `watchers` VALUES (10054,14,'2025-11-26 15:03:21.000');
INSERT INTO `watchers` VALUES (10054,16,'2025-11-22 16:06:40.000');
INSERT INTO `watchers` VALUES (10055,6,'2025-11-22 17:00:29.000');
INSERT INTO `watchers` VALUES (10055,17,'2025-11-27 14:23:12.000');
INSERT INTO `watchers` VALUES (10056,4,'2025-11-28 09:29:54.000');
INSERT INTO `watchers` VALUES (10056,5,'2025-11-30 12:29:18.000');
INSERT INTO `watchers` VALUES (10056,6,'2025-11-27 09:20:21.000');
INSERT INTO `watchers` VALUES (10056,16,'2025-11-27 11:09:16.000');
INSERT INTO `watchers` VALUES (10057,7,'2025-11-30 16:51:27.000');
INSERT INTO `watchers` VALUES (10057,9,'2025-12-02 07:02:36.000');
INSERT INTO `watchers` VALUES (10057,21,'2025-12-01 12:05:05.000');
INSERT INTO `watchers` VALUES (10057,22,'2025-11-29 16:40:34.000');
INSERT INTO `watchers` VALUES (10058,5,'2025-11-30 07:04:45.000');
INSERT INTO `watchers` VALUES (10058,7,'2025-11-29 12:44:31.000');
INSERT INTO `watchers` VALUES (10058,15,'2025-12-01 09:39:12.000');
INSERT INTO `watchers` VALUES (10059,16,'2025-12-01 08:13:42.000');
INSERT INTO `watchers` VALUES (10060,6,'2025-12-06 10:46:52.000');
INSERT INTO `watchers` VALUES (10061,3,'2025-12-03 12:50:06.000');
INSERT INTO `watchers` VALUES (10061,4,'2025-12-02 17:06:51.000');
INSERT INTO `watchers` VALUES (10061,8,'2025-12-06 18:58:21.000');
INSERT INTO `watchers` VALUES (10061,21,'2025-12-04 08:56:17.000');
INSERT INTO `watchers` VALUES (10062,4,'2025-12-04 16:48:51.000');
INSERT INTO `watchers` VALUES (10062,5,'2025-12-08 17:52:50.000');
INSERT INTO `watchers` VALUES (10062,15,'2025-12-05 07:42:18.000');
INSERT INTO `watchers` VALUES (10062,17,'2025-12-08 09:40:24.000');
INSERT INTO `watchers` VALUES (10063,2,'2025-12-08 18:50:55.000');
INSERT INTO `watchers` VALUES (10063,4,'2025-12-06 10:23:10.000');
INSERT INTO `watchers` VALUES (10064,6,'2025-12-09 14:10:16.000');
INSERT INTO `watchers` VALUES (10064,11,'2025-12-09 15:18:46.000');
INSERT INTO `watchers` VALUES (10064,16,'2025-12-08 15:03:27.000');
INSERT INTO `watchers` VALUES (10064,17,'2025-12-08 13:23:37.000');
INSERT INTO `watchers` VALUES (10065,9,'2025-12-07 12:51:57.000');
INSERT INTO `watchers` VALUES (10066,11,'2025-12-12 15:03:11.000');
INSERT INTO `watchers` VALUES (10066,23,'2025-12-12 15:12:50.000');
INSERT INTO `watchers` VALUES (10067,6,'2025-12-10 13:42:59.000');
INSERT INTO `watchers` VALUES (10067,9,'2025-12-12 09:24:18.000');
INSERT INTO `watchers` VALUES (10067,11,'2025-12-12 14:18:44.000');
INSERT INTO `watchers` VALUES (10067,23,'2025-12-11 11:18:01.000');
INSERT INTO `watchers` VALUES (10068,7,'2025-12-12 14:04:04.000');
INSERT INTO `watchers` VALUES (10068,10,'2025-12-12 07:27:08.000');
INSERT INTO `watchers` VALUES (10069,3,'2025-12-14 11:19:48.000');
INSERT INTO `watchers` VALUES (10069,4,'2025-12-14 14:20:47.000');
INSERT INTO `watchers` VALUES (10069,7,'2025-12-11 11:49:49.000');
INSERT INTO `watchers` VALUES (10069,9,'2025-12-09 11:40:50.000');
INSERT INTO `watchers` VALUES (10070,3,'2025-12-12 09:51:37.000');
INSERT INTO `watchers` VALUES (10070,10,'2025-12-14 09:40:18.000');
INSERT INTO `watchers` VALUES (10070,11,'2025-12-10 11:59:37.000');
INSERT INTO `watchers` VALUES (10070,23,'2025-12-11 13:41:09.000');
INSERT INTO `watchers` VALUES (10071,8,'2025-12-13 17:18:45.000');
INSERT INTO `watchers` VALUES (10071,10,'2025-12-14 17:48:52.000');
INSERT INTO `watchers` VALUES (10071,15,'2025-12-13 16:20:35.000');
INSERT INTO `watchers` VALUES (10072,2,'2025-12-12 15:29:15.000');
INSERT INTO `watchers` VALUES (10072,7,'2025-12-10 08:30:41.000');
INSERT INTO `watchers` VALUES (10072,8,'2025-12-13 14:08:06.000');
INSERT INTO `watchers` VALUES (10072,23,'2025-12-09 18:40:49.000');
INSERT INTO `watchers` VALUES (10073,4,'2025-12-14 08:50:44.000');
INSERT INTO `watchers` VALUES (10074,11,'2025-12-13 18:38:56.000');
INSERT INTO `watchers` VALUES (10074,13,'2025-12-13 18:34:42.000');
INSERT INTO `watchers` VALUES (10074,17,'2025-12-14 16:01:25.000');
INSERT INTO `watchers` VALUES (10075,3,'2025-12-17 16:27:33.000');
INSERT INTO `watchers` VALUES (10075,17,'2025-12-16 16:56:23.000');
INSERT INTO `watchers` VALUES (10076,2,'2025-12-19 11:06:08.000');
INSERT INTO `watchers` VALUES (10076,3,'2025-12-17 12:24:40.000');
INSERT INTO `watchers` VALUES (10077,1,'2025-12-17 14:20:41.000');
INSERT INTO `watchers` VALUES (10077,16,'2025-12-19 08:39:00.000');
INSERT INTO `watchers` VALUES (10078,8,'2025-12-20 14:09:52.000');
INSERT INTO `watchers` VALUES (10078,14,'2025-12-18 15:55:09.000');
INSERT INTO `watchers` VALUES (10079,8,'2025-12-21 07:43:53.000');
INSERT INTO `watchers` VALUES (10079,14,'2025-12-22 12:59:05.000');
INSERT INTO `watchers` VALUES (10080,7,'2025-12-20 15:04:13.000');
INSERT INTO `watchers` VALUES (10080,16,'2025-12-24 12:25:34.000');
INSERT INTO `watchers` VALUES (10081,9,'2025-12-28 12:02:59.000');
INSERT INTO `watchers` VALUES (10081,13,'2025-12-25 07:46:46.000');
INSERT INTO `watchers` VALUES (10081,17,'2025-12-24 15:28:24.000');
INSERT INTO `watchers` VALUES (10082,10,'2025-12-28 16:33:05.000');
INSERT INTO `watchers` VALUES (10082,14,'2025-12-29 09:53:41.000');
INSERT INTO `watchers` VALUES (10082,23,'2025-12-30 08:25:28.000');
INSERT INTO `watchers` VALUES (10083,2,'2025-12-30 12:49:17.000');
INSERT INTO `watchers` VALUES (10083,7,'2025-12-31 12:33:04.000');
INSERT INTO `watchers` VALUES (10084,7,'2025-12-28 14:21:16.000');
INSERT INTO `watchers` VALUES (10084,15,'2025-12-28 17:01:48.000');
INSERT INTO `watchers` VALUES (10084,19,'2025-12-28 13:49:52.000');
INSERT INTO `watchers` VALUES (10085,3,'2025-12-30 10:25:25.000');
INSERT INTO `watchers` VALUES (10085,9,'2025-12-30 13:51:06.000');
INSERT INTO `watchers` VALUES (10085,10,'2025-12-31 15:24:04.000');
INSERT INTO `watchers` VALUES (10085,19,'2025-12-30 17:03:59.000');
INSERT INTO `watchers` VALUES (10086,7,'2026-01-05 11:28:40.000');
INSERT INTO `watchers` VALUES (10086,11,'2026-01-04 16:16:54.000');
INSERT INTO `watchers` VALUES (10086,21,'2026-01-03 09:32:03.000');
INSERT INTO `watchers` VALUES (10087,11,'2026-01-06 10:37:25.000');
INSERT INTO `watchers` VALUES (10087,20,'2026-01-03 13:07:20.000');
INSERT INTO `watchers` VALUES (10088,2,'2026-01-06 14:44:58.000');
INSERT INTO `watchers` VALUES (10088,9,'2026-01-07 07:55:40.000');
INSERT INTO `watchers` VALUES (10088,13,'2026-01-08 18:18:25.000');
INSERT INTO `watchers` VALUES (10088,17,'2026-01-04 08:22:02.000');
INSERT INTO `watchers` VALUES (10089,6,'2026-01-06 08:24:02.000');
INSERT INTO `watchers` VALUES (10089,10,'2026-01-07 15:22:07.000');
INSERT INTO `watchers` VALUES (10089,14,'2026-01-09 10:52:34.000');
INSERT INTO `watchers` VALUES (10090,3,'2026-01-06 17:15:07.000');
INSERT INTO `watchers` VALUES (10090,21,'2026-01-06 07:36:40.000');
INSERT INTO `watchers` VALUES (10090,23,'2026-01-09 14:52:53.000');
INSERT INTO `watchers` VALUES (10091,11,'2026-01-07 09:12:17.000');
INSERT INTO `watchers` VALUES (10092,4,'2026-01-12 17:15:17.000');
INSERT INTO `watchers` VALUES (10092,11,'2026-01-12 12:20:48.000');
INSERT INTO `watchers` VALUES (10093,5,'2026-01-21 12:44:07.000');
INSERT INTO `watchers` VALUES (10093,7,'2026-01-19 10:57:51.000');
INSERT INTO `watchers` VALUES (10094,5,'2026-01-23 16:27:36.000');
INSERT INTO `watchers` VALUES (10094,15,'2026-01-23 14:24:57.000');
INSERT INTO `watchers` VALUES (10094,22,'2026-01-22 13:59:05.000');
INSERT INTO `watchers` VALUES (10095,4,'2026-01-26 17:51:50.000');
INSERT INTO `watchers` VALUES (10095,7,'2026-01-26 07:06:12.000');
INSERT INTO `watchers` VALUES (10095,8,'2026-01-26 16:15:21.000');
INSERT INTO `watchers` VALUES (10095,14,'2026-01-27 07:29:39.000');
INSERT INTO `watchers` VALUES (10096,7,'2026-01-27 14:40:20.000');
INSERT INTO `watchers` VALUES (10096,15,'2026-01-28 16:06:07.000');
INSERT INTO `watchers` VALUES (10097,10,'2026-01-29 13:16:44.000');
INSERT INTO `watchers` VALUES (10097,14,'2026-01-26 16:47:25.000');
INSERT INTO `watchers` VALUES (10098,3,'2026-01-30 12:12:49.000');
INSERT INTO `watchers` VALUES (10098,4,'2026-01-29 09:54:15.000');
INSERT INTO `watchers` VALUES (10098,8,'2026-01-27 14:26:55.000');
INSERT INTO `watchers` VALUES (10099,15,'2026-02-01 16:43:40.000');
INSERT INTO `watchers` VALUES (10100,12,'2026-01-29 12:11:41.000');
INSERT INTO `watchers` VALUES (10100,15,'2026-01-29 09:34:20.000');
INSERT INTO `watchers` VALUES (10101,6,'2026-02-02 17:27:26.000');
INSERT INTO `watchers` VALUES (10101,9,'2026-01-31 13:59:52.000');
INSERT INTO `watchers` VALUES (10102,3,'2026-02-03 18:51:51.000');
INSERT INTO `watchers` VALUES (10102,17,'2026-01-30 18:55:40.000');
INSERT INTO `watchers` VALUES (10102,21,'2026-02-04 17:27:53.000');
INSERT INTO `watchers` VALUES (10102,22,'2026-01-30 18:25:06.000');
INSERT INTO `watchers` VALUES (10103,6,'2026-02-03 16:39:43.000');
INSERT INTO `watchers` VALUES (10104,7,'2026-02-06 17:19:34.000');
INSERT INTO `watchers` VALUES (10104,11,'2026-02-03 18:18:12.000');
INSERT INTO `watchers` VALUES (10104,16,'2026-02-05 13:19:09.000');
INSERT INTO `watchers` VALUES (10105,11,'2026-02-04 16:18:19.000');
INSERT INTO `watchers` VALUES (10106,10,'2026-02-07 11:50:16.000');
INSERT INTO `watchers` VALUES (10106,14,'2026-02-05 13:41:27.000');
INSERT INTO `watchers` VALUES (10107,1,'2026-02-08 14:02:55.000');
INSERT INTO `watchers` VALUES (10107,4,'2026-02-05 18:36:14.000');
INSERT INTO `watchers` VALUES (10107,13,'2026-02-06 12:36:47.000');
INSERT INTO `watchers` VALUES (10107,14,'2026-02-05 09:39:52.000');
INSERT INTO `watchers` VALUES (10108,3,'2026-02-05 13:54:51.000');
INSERT INTO `watchers` VALUES (10108,10,'2026-02-07 17:04:48.000');
INSERT INTO `watchers` VALUES (10108,14,'2026-02-04 17:02:16.000');
INSERT INTO `watchers` VALUES (10109,10,'2026-02-12 17:50:23.000');
INSERT INTO `watchers` VALUES (10109,11,'2026-02-11 11:03:14.000');
INSERT INTO `watchers` VALUES (10109,16,'2026-02-07 18:05:54.000');
INSERT INTO `watchers` VALUES (10110,9,'2026-02-10 17:09:56.000');
INSERT INTO `watchers` VALUES (10110,13,'2026-02-08 18:13:28.000');
INSERT INTO `watchers` VALUES (10110,16,'2026-02-12 11:29:23.000');
INSERT INTO `watchers` VALUES (10111,2,'2026-02-08 14:15:11.000');
INSERT INTO `watchers` VALUES (10111,10,'2026-02-08 16:41:50.000');
INSERT INTO `watchers` VALUES (10111,11,'2026-02-10 12:05:31.000');
INSERT INTO `watchers` VALUES (10112,3,'2026-02-10 12:29:09.000');
INSERT INTO `watchers` VALUES (10112,5,'2026-02-13 07:41:34.000');
INSERT INTO `watchers` VALUES (10112,16,'2026-02-13 07:04:00.000');
INSERT INTO `watchers` VALUES (10113,11,'2026-02-14 08:58:09.000');
INSERT INTO `watchers` VALUES (10113,21,'2026-02-13 16:35:33.000');
INSERT INTO `watchers` VALUES (10114,10,'2026-02-16 13:33:00.000');
INSERT INTO `watchers` VALUES (10115,8,'2026-02-17 14:29:39.000');
INSERT INTO `watchers` VALUES (10115,14,'2026-02-16 12:30:34.000');
INSERT INTO `watchers` VALUES (10115,21,'2026-02-13 18:06:00.000');
INSERT INTO `watchers` VALUES (10116,7,'2026-02-15 18:21:28.000');
INSERT INTO `watchers` VALUES (10116,12,'2026-02-18 14:51:55.000');
INSERT INTO `watchers` VALUES (10116,15,'2026-02-15 10:03:37.000');
INSERT INTO `watchers` VALUES (10117,13,'2026-02-17 10:14:13.000');
INSERT INTO `watchers` VALUES (10117,15,'2026-02-21 11:06:57.000');
INSERT INTO `watchers` VALUES (10118,7,'2026-02-18 18:54:50.000');
INSERT INTO `watchers` VALUES (10118,12,'2026-02-18 18:59:00.000');
INSERT INTO `watchers` VALUES (10119,2,'2026-02-20 15:22:03.000');
INSERT INTO `watchers` VALUES (10119,4,'2026-02-20 17:50:51.000');
INSERT INTO `watchers` VALUES (10119,15,'2026-02-23 17:51:39.000');
INSERT INTO `watchers` VALUES (10120,5,'2026-02-26 16:09:50.000');
INSERT INTO `watchers` VALUES (10120,9,'2026-02-22 17:46:12.000');
INSERT INTO `watchers` VALUES (10121,1,'2026-02-28 14:03:21.000');
INSERT INTO `watchers` VALUES (10121,7,'2026-02-27 12:32:19.000');
INSERT INTO `watchers` VALUES (10122,2,'2026-02-24 13:16:24.000');
INSERT INTO `watchers` VALUES (10122,5,'2026-02-25 14:25:11.000');
INSERT INTO `watchers` VALUES (10122,17,'2026-02-28 08:29:46.000');
INSERT INTO `watchers` VALUES (10123,5,'2026-02-25 07:29:12.000');
INSERT INTO `watchers` VALUES (10123,15,'2026-02-24 07:47:23.000');
INSERT INTO `watchers` VALUES (10123,23,'2026-02-24 08:16:36.000');
INSERT INTO `watchers` VALUES (10124,3,'2026-02-28 11:49:35.000');
INSERT INTO `watchers` VALUES (10124,11,'2026-02-25 10:01:00.000');
INSERT INTO `watchers` VALUES (10124,12,'2026-02-27 12:18:44.000');
INSERT INTO `watchers` VALUES (10124,15,'2026-02-28 07:05:37.000');
INSERT INTO `watchers` VALUES (10125,2,'2026-02-26 17:12:22.000');
INSERT INTO `watchers` VALUES (10125,19,'2026-02-25 12:50:43.000');
INSERT INTO `watchers` VALUES (10125,22,'2026-03-01 11:54:48.000');
INSERT INTO `watchers` VALUES (10126,4,'2026-02-26 08:04:50.000');
INSERT INTO `watchers` VALUES (10126,12,'2026-03-01 17:02:11.000');
INSERT INTO `watchers` VALUES (10127,5,'2026-02-28 09:29:22.000');
INSERT INTO `watchers` VALUES (10127,6,'2026-03-03 08:50:24.000');
INSERT INTO `watchers` VALUES (10127,9,'2026-03-03 09:36:54.000');
INSERT INTO `watchers` VALUES (10127,14,'2026-02-28 09:02:09.000');
INSERT INTO `watchers` VALUES (10128,19,'2026-03-02 16:20:29.000');
INSERT INTO `watchers` VALUES (10128,20,'2026-03-03 09:00:29.000');
INSERT INTO `watchers` VALUES (10129,18,'2026-03-04 07:48:06.000');
INSERT INTO `watchers` VALUES (10130,1,'2026-03-06 09:04:18.000');
INSERT INTO `watchers` VALUES (10130,3,'2026-03-04 12:51:50.000');
INSERT INTO `watchers` VALUES (10130,7,'2026-03-02 11:47:02.000');
INSERT INTO `watchers` VALUES (10130,22,'2026-03-03 13:55:41.000');
INSERT INTO `watchers` VALUES (10131,4,'2026-03-04 08:10:24.000');
INSERT INTO `watchers` VALUES (10131,5,'2026-03-06 12:12:17.000');
INSERT INTO `watchers` VALUES (10132,7,'2026-03-13 16:42:09.000');
INSERT INTO `watchers` VALUES (10132,10,'2026-03-10 16:11:44.000');
INSERT INTO `watchers` VALUES (10132,19,'2026-03-11 15:11:42.000');
INSERT INTO `watchers` VALUES (10132,23,'2026-03-10 16:00:06.000');
INSERT INTO `watchers` VALUES (10133,4,'2026-03-15 08:29:31.000');
INSERT INTO `watchers` VALUES (10133,6,'2026-03-15 17:30:31.000');
INSERT INTO `watchers` VALUES (10133,13,'2026-03-15 11:47:40.000');
INSERT INTO `watchers` VALUES (10134,5,'2026-03-13 07:48:14.000');
INSERT INTO `watchers` VALUES (10134,16,'2026-03-15 13:00:32.000');
INSERT INTO `watchers` VALUES (10134,22,'2026-03-14 15:51:43.000');
INSERT INTO `watchers` VALUES (10134,23,'2026-03-15 13:36:03.000');
INSERT INTO `watchers` VALUES (10135,3,'2026-03-15 18:45:07.000');
INSERT INTO `watchers` VALUES (10135,11,'2026-03-13 12:34:40.000');
INSERT INTO `watchers` VALUES (10136,4,'2026-03-19 09:15:40.000');
INSERT INTO `watchers` VALUES (10136,9,'2026-03-17 09:20:21.000');
INSERT INTO `watchers` VALUES (10136,14,'2026-03-18 08:55:46.000');
INSERT INTO `watchers` VALUES (10136,22,'2026-03-18 12:34:00.000');
INSERT INTO `watchers` VALUES (10137,4,'2026-03-18 07:04:47.000');
INSERT INTO `watchers` VALUES (10137,21,'2026-03-15 08:20:17.000');
INSERT INTO `watchers` VALUES (10138,5,'2026-03-14 18:32:05.000');
INSERT INTO `watchers` VALUES (10138,20,'2026-03-16 13:11:21.000');
INSERT INTO `watchers` VALUES (10139,16,'2026-03-14 18:51:12.000');
INSERT INTO `watchers` VALUES (10139,19,'2026-03-15 15:41:56.000');
INSERT INTO `watchers` VALUES (10140,1,'2026-03-21 10:20:27.000');
INSERT INTO `watchers` VALUES (10140,2,'2026-03-19 08:54:23.000');
INSERT INTO `watchers` VALUES (10140,6,'2026-03-20 11:05:41.000');
INSERT INTO `watchers` VALUES (10140,18,'2026-03-22 15:08:05.000');
INSERT INTO `watchers` VALUES (10141,5,'2026-03-20 16:06:34.000');
INSERT INTO `watchers` VALUES (10141,9,'2026-03-24 12:10:05.000');
INSERT INTO `watchers` VALUES (10141,11,'2026-03-20 12:09:31.000');
INSERT INTO `watchers` VALUES (10142,7,'2026-03-24 13:41:50.000');
INSERT INTO `watchers` VALUES (10142,16,'2026-03-21 14:25:04.000');
INSERT INTO `watchers` VALUES (10143,1,'2026-03-23 08:04:05.000');
INSERT INTO `watchers` VALUES (10143,9,'2026-03-22 08:42:44.000');
INSERT INTO `watchers` VALUES (10143,11,'2026-03-23 15:55:40.000');
INSERT INTO `watchers` VALUES (10143,22,'2026-03-20 07:00:51.000');
INSERT INTO `watchers` VALUES (10144,11,'2026-03-25 15:38:04.000');
INSERT INTO `watchers` VALUES (10144,22,'2026-03-25 13:04:05.000');
INSERT INTO `watchers` VALUES (10144,23,'2026-03-20 18:20:10.000');
INSERT INTO `watchers` VALUES (10145,6,'2026-03-27 08:50:13.000');
INSERT INTO `watchers` VALUES (10145,11,'2026-03-26 07:53:00.000');
INSERT INTO `watchers` VALUES (10145,19,'2026-03-24 12:33:15.000');
INSERT INTO `watchers` VALUES (10146,2,'2026-03-28 12:16:10.000');
INSERT INTO `watchers` VALUES (10146,11,'2026-03-30 06:12:52.000');
INSERT INTO `watchers` VALUES (10146,13,'2026-03-27 12:07:28.000');
INSERT INTO `watchers` VALUES (10147,1,'2026-03-30 09:16:09.000');
INSERT INTO `watchers` VALUES (10147,8,'2026-03-31 14:11:22.000');
INSERT INTO `watchers` VALUES (10147,10,'2026-03-27 11:39:08.000');
INSERT INTO `watchers` VALUES (10147,17,'2026-03-31 14:21:57.000');
INSERT INTO `watchers` VALUES (10148,8,'2026-03-29 12:22:21.000');
INSERT INTO `watchers` VALUES (10148,11,'2026-03-28 15:31:11.000');
INSERT INTO `watchers` VALUES (10148,19,'2026-04-02 10:49:48.000');
INSERT INTO `watchers` VALUES (10149,7,'2026-04-04 15:40:00.000');
INSERT INTO `watchers` VALUES (10149,11,'2026-04-08 11:54:17.000');
INSERT INTO `watchers` VALUES (10149,15,'2026-04-08 08:03:16.000');
INSERT INTO `watchers` VALUES (10150,2,'2026-04-09 12:05:35.000');
INSERT INTO `watchers` VALUES (10150,10,'2026-04-06 12:31:51.000');
INSERT INTO `watchers` VALUES (10150,14,'2026-04-06 12:40:40.000');
INSERT INTO `watchers` VALUES (10151,10,'2026-04-07 16:00:13.000');
INSERT INTO `watchers` VALUES (10151,14,'2026-04-07 09:22:01.000');
INSERT INTO `watchers` VALUES (10152,4,'2026-04-08 07:24:02.000');
INSERT INTO `watchers` VALUES (10152,9,'2026-04-08 10:21:48.000');
INSERT INTO `watchers` VALUES (10152,10,'2026-04-08 15:11:36.000');
INSERT INTO `watchers` VALUES (10152,17,'2026-04-09 14:36:16.000');
INSERT INTO `watchers` VALUES (10153,1,'2026-04-14 08:05:49.000');
INSERT INTO `watchers` VALUES (10153,2,'2026-04-15 17:34:52.000');
INSERT INTO `watchers` VALUES (10153,5,'2026-04-12 08:32:07.000');
INSERT INTO `watchers` VALUES (10153,15,'2026-04-12 15:12:03.000');
INSERT INTO `watchers` VALUES (10154,11,'2026-04-12 15:36:07.000');
INSERT INTO `watchers` VALUES (10154,17,'2026-04-14 08:24:42.000');
INSERT INTO `watchers` VALUES (10154,20,'2026-04-11 17:27:38.000');
INSERT INTO `watchers` VALUES (10155,10,'2026-04-12 16:50:52.000');
INSERT INTO `watchers` VALUES (10155,17,'2026-04-14 09:37:07.000');
INSERT INTO `watchers` VALUES (10156,6,'2026-04-14 12:51:59.000');
INSERT INTO `watchers` VALUES (10156,12,'2026-04-16 14:29:26.000');
INSERT INTO `watchers` VALUES (10156,15,'2026-04-19 06:08:00.000');
INSERT INTO `watchers` VALUES (10157,8,'2026-04-15 12:33:04.000');
INSERT INTO `watchers` VALUES (10157,11,'2026-04-16 12:30:28.000');
INSERT INTO `watchers` VALUES (10158,3,'2026-04-17 16:02:12.000');
INSERT INTO `watchers` VALUES (10158,17,'2026-04-16 09:46:11.000');
INSERT INTO `watchers` VALUES (10159,2,'2026-04-18 08:18:01.000');
INSERT INTO `watchers` VALUES (10159,7,'2026-04-18 09:24:32.000');
INSERT INTO `watchers` VALUES (10159,9,'2026-04-18 10:31:50.000');
INSERT INTO `watchers` VALUES (10160,11,'2026-04-20 16:39:50.000');
INSERT INTO `watchers` VALUES (10161,11,'2026-04-17 16:52:31.000');
INSERT INTO `watchers` VALUES (10161,17,'2026-04-17 13:49:14.000');
INSERT INTO `watchers` VALUES (10161,21,'2026-04-17 14:36:06.000');
INSERT INTO `watchers` VALUES (10162,9,'2026-04-19 15:04:16.000');
INSERT INTO `watchers` VALUES (10163,2,'2026-04-18 13:39:03.000');
INSERT INTO `watchers` VALUES (10163,22,'2026-04-21 14:50:19.000');
INSERT INTO `watchers` VALUES (10164,2,'2026-04-22 14:03:50.000');
INSERT INTO `watchers` VALUES (10164,5,'2026-04-23 14:01:35.000');
INSERT INTO `watchers` VALUES (10164,11,'2026-04-23 06:01:14.000');
INSERT INTO `watchers` VALUES (10164,15,'2026-04-21 10:08:30.000');
INSERT INTO `watchers` VALUES (10165,10,'2026-04-21 08:23:41.000');
INSERT INTO `watchers` VALUES (10165,15,'2026-04-22 07:08:14.000');
INSERT INTO `watchers` VALUES (10165,19,'2026-04-21 13:45:13.000');
INSERT INTO `watchers` VALUES (10166,5,'2026-04-21 16:28:11.000');
INSERT INTO `watchers` VALUES (10166,7,'2026-04-24 13:04:37.000');
INSERT INTO `watchers` VALUES (10166,23,'2026-04-26 16:47:50.000');
INSERT INTO `watchers` VALUES (10167,2,'2026-04-23 14:16:19.000');
INSERT INTO `watchers` VALUES (10167,10,'2026-04-23 11:29:55.000');
INSERT INTO `watchers` VALUES (10167,17,'2026-04-23 10:12:58.000');
INSERT INTO `watchers` VALUES (10168,9,'2026-04-23 13:29:15.000');
INSERT INTO `watchers` VALUES (10168,13,'2026-04-26 14:28:46.000');
INSERT INTO `watchers` VALUES (10168,18,'2026-04-22 15:31:49.000');
INSERT INTO `watchers` VALUES (10168,23,'2026-04-24 13:47:11.000');
INSERT INTO `watchers` VALUES (10169,3,'2026-04-28 16:46:39.000');
INSERT INTO `watchers` VALUES (10169,13,'2026-04-28 17:48:44.000');
INSERT INTO `watchers` VALUES (10170,12,'2026-04-28 14:08:18.000');
INSERT INTO `watchers` VALUES (10170,13,'2026-04-23 15:47:31.000');
INSERT INTO `watchers` VALUES (10170,14,'2026-04-28 09:33:18.000');
INSERT INTO `watchers` VALUES (10171,8,'2026-04-27 13:10:52.000');
INSERT INTO `watchers` VALUES (10171,12,'2026-04-28 11:13:42.000');
INSERT INTO `watchers` VALUES (10171,14,'2026-04-26 16:18:54.000');
INSERT INTO `watchers` VALUES (10171,23,'2026-04-27 08:07:27.000');
INSERT INTO `watchers` VALUES (10172,6,'2026-04-28 16:17:35.000');
INSERT INTO `watchers` VALUES (10172,10,'2026-05-01 07:40:44.000');
INSERT INTO `watchers` VALUES (10172,11,'2026-04-27 07:29:43.000');
INSERT INTO `watchers` VALUES (10172,15,'2026-04-28 07:06:53.000');
INSERT INTO `watchers` VALUES (10173,6,'2026-05-03 15:10:42.000');
INSERT INTO `watchers` VALUES (10173,15,'2026-04-30 10:49:52.000');
INSERT INTO `watchers` VALUES (10173,16,'2026-05-02 09:40:48.000');
INSERT INTO `watchers` VALUES (10174,2,'2026-05-04 09:27:43.000');
INSERT INTO `watchers` VALUES (10174,5,'2026-04-29 13:12:03.000');
INSERT INTO `watchers` VALUES (10175,1,'2026-05-07 16:22:55.000');
INSERT INTO `watchers` VALUES (10175,7,'2026-05-06 07:09:10.000');
INSERT INTO `watchers` VALUES (10175,15,'2026-05-06 16:23:59.000');
INSERT INTO `watchers` VALUES (10175,21,'2026-05-06 06:41:15.000');
INSERT INTO `watchers` VALUES (10176,4,'2026-05-09 09:14:03.000');
INSERT INTO `watchers` VALUES (10176,10,'2026-05-04 12:12:25.000');
INSERT INTO `watchers` VALUES (10176,12,'2026-05-08 06:57:13.000');
INSERT INTO `watchers` VALUES (10176,13,'2026-05-04 12:15:17.000');
INSERT INTO `watchers` VALUES (10177,7,'2026-05-05 14:51:21.000');
INSERT INTO `watchers` VALUES (10177,12,'2026-05-09 09:02:41.000');
INSERT INTO `watchers` VALUES (10177,22,'2026-05-07 08:47:55.000');
INSERT INTO `watchers` VALUES (10178,2,'2026-05-13 17:14:42.000');
INSERT INTO `watchers` VALUES (10178,11,'2026-05-11 16:53:05.000');
INSERT INTO `watchers` VALUES (10178,19,'2026-05-12 15:36:37.000');
INSERT INTO `watchers` VALUES (10179,2,'2026-05-16 17:14:45.000');
INSERT INTO `watchers` VALUES (10179,4,'2026-05-12 12:22:57.000');
INSERT INTO `watchers` VALUES (10179,6,'2026-05-15 13:18:42.000');
INSERT INTO `watchers` VALUES (10179,20,'2026-05-17 08:20:28.000');
INSERT INTO `watchers` VALUES (10180,14,'2026-05-17 17:44:10.000');
INSERT INTO `watchers` VALUES (10180,17,'2026-05-15 07:50:43.000');
INSERT INTO `watchers` VALUES (10181,16,'2026-05-12 16:53:10.000');
INSERT INTO `watchers` VALUES (10181,20,'2026-05-12 16:23:20.000');
INSERT INTO `watchers` VALUES (10182,7,'2026-05-13 17:21:09.000');
INSERT INTO `watchers` VALUES (10183,4,'2026-05-13 16:09:38.000');
INSERT INTO `watchers` VALUES (10183,7,'2026-05-17 12:43:22.000');
INSERT INTO `watchers` VALUES (10183,22,'2026-05-18 16:35:59.000');
INSERT INTO `watchers` VALUES (10184,7,'2026-05-15 16:18:30.000');
INSERT INTO `watchers` VALUES (10184,16,'2026-05-18 06:54:27.000');
INSERT INTO `watchers` VALUES (10185,5,'2026-05-17 13:10:50.000');
INSERT INTO `watchers` VALUES (10185,15,'2026-05-16 11:20:41.000');
INSERT INTO `watchers` VALUES (10185,18,'2026-05-17 13:07:16.000');
INSERT INTO `watchers` VALUES (10186,13,'2026-05-15 17:57:32.000');
INSERT INTO `watchers` VALUES (10186,15,'2026-05-15 17:49:19.000');
INSERT INTO `watchers` VALUES (10186,17,'2026-05-19 13:40:36.000');
INSERT INTO `watchers` VALUES (10187,9,'2026-05-20 08:03:30.000');
INSERT INTO `watchers` VALUES (10187,19,'2026-05-16 17:31:48.000');
INSERT INTO `watchers` VALUES (10187,22,'2026-05-18 12:21:31.000');
INSERT INTO `watchers` VALUES (10188,3,'2026-05-22 10:04:01.000');
INSERT INTO `watchers` VALUES (10188,6,'2026-05-22 15:28:29.000');
INSERT INTO `watchers` VALUES (10188,12,'2026-05-21 12:29:30.000');
INSERT INTO `watchers` VALUES (10189,2,'2026-05-19 16:37:49.000');
INSERT INTO `watchers` VALUES (10189,3,'2026-05-19 11:57:00.000');
INSERT INTO `watchers` VALUES (10189,11,'2026-05-22 09:13:27.000');
INSERT INTO `watchers` VALUES (10190,5,'2026-05-21 09:19:40.000');
INSERT INTO `watchers` VALUES (10190,11,'2026-05-20 06:24:17.000');
INSERT INTO `watchers` VALUES (10191,21,'2026-05-18 12:58:01.000');
INSERT INTO `watchers` VALUES (10191,22,'2026-05-23 16:24:21.000');
INSERT INTO `watchers` VALUES (10192,7,'2026-05-24 13:16:02.000');
INSERT INTO `watchers` VALUES (10192,16,'2026-05-20 14:30:32.000');
INSERT INTO `watchers` VALUES (10192,19,'2026-05-24 14:50:33.000');
INSERT INTO `watchers` VALUES (10193,10,'2026-05-20 11:41:51.000');
INSERT INTO `watchers` VALUES (10193,12,'2026-05-20 15:44:45.000');
INSERT INTO `watchers` VALUES (10193,17,'2026-05-21 16:55:18.000');
INSERT INTO `watchers` VALUES (10194,5,'2026-05-25 16:30:44.000');
INSERT INTO `watchers` VALUES (10194,8,'2026-05-22 17:16:19.000');
INSERT INTO `watchers` VALUES (10194,23,'2026-05-21 07:35:52.000');
INSERT INTO `watchers` VALUES (10195,10,'2026-05-22 12:13:26.000');
INSERT INTO `watchers` VALUES (10195,13,'2026-05-26 14:53:35.000');
INSERT INTO `watchers` VALUES (10195,15,'2026-05-22 11:14:54.000');
INSERT INTO `watchers` VALUES (10196,6,'2026-05-24 08:30:26.000');
INSERT INTO `watchers` VALUES (10196,19,'2026-05-22 15:40:24.000');
INSERT INTO `watchers` VALUES (10197,8,'2026-05-24 12:01:37.000');
INSERT INTO `watchers` VALUES (10197,10,'2026-05-26 13:19:48.000');
INSERT INTO `watchers` VALUES (10197,14,'2026-05-26 07:31:17.000');
INSERT INTO `watchers` VALUES (10197,20,'2026-05-29 17:13:17.000');
INSERT INTO `watchers` VALUES (10198,3,'2026-05-24 17:52:52.000');
INSERT INTO `watchers` VALUES (10198,19,'2026-05-24 14:47:07.000');
INSERT INTO `watchers` VALUES (10199,3,'2026-05-28 10:55:18.000');
INSERT INTO `watchers` VALUES (10199,11,'2026-05-28 11:04:23.000');
INSERT INTO `watchers` VALUES (10200,2,'2026-05-26 12:03:28.000');
INSERT INTO `watchers` VALUES (10200,4,'2026-05-27 14:42:59.000');
INSERT INTO `watchers` VALUES (10200,9,'2026-05-28 06:16:24.000');
INSERT INTO `watchers` VALUES (10200,16,'2026-05-29 14:49:57.000');
INSERT INTO `watchers` VALUES (10201,7,'2026-05-26 13:35:22.000');
INSERT INTO `watchers` VALUES (10201,9,'2026-05-30 17:52:41.000');
INSERT INTO `watchers` VALUES (10202,4,'2026-05-30 11:20:02.000');
INSERT INTO `watchers` VALUES (10202,5,'2026-05-27 09:44:42.000');
INSERT INTO `watchers` VALUES (10202,11,'2026-05-30 15:27:29.000');
INSERT INTO `watchers` VALUES (10203,10,'2026-05-29 08:11:12.000');
INSERT INTO `watchers` VALUES (10203,14,'2026-05-31 15:49:52.000');
INSERT INTO `watchers` VALUES (10204,11,'2026-05-28 10:36:00.000');
INSERT INTO `watchers` VALUES (10204,12,'2026-06-01 12:36:52.000');
INSERT INTO `watchers` VALUES (10205,23,'2026-06-03 09:35:46.000');
INSERT INTO `watchers` VALUES (10206,9,'2026-05-31 06:29:55.000');
INSERT INTO `watchers` VALUES (10206,12,'2026-05-30 11:57:58.000');
INSERT INTO `watchers` VALUES (10206,21,'2026-05-31 06:53:02.000');
INSERT INTO `watchers` VALUES (10207,1,'2026-06-04 11:46:07.000');
INSERT INTO `watchers` VALUES (10207,16,'2026-06-03 15:57:05.000');
INSERT INTO `watchers` VALUES (10207,17,'2026-05-30 10:43:57.000');
INSERT INTO `watchers` VALUES (10207,19,'2026-05-30 17:22:23.000');
INSERT INTO `watchers` VALUES (10208,10,'2026-06-02 16:26:15.000');
INSERT INTO `watchers` VALUES (10208,17,'2026-06-03 15:24:19.000');
INSERT INTO `watchers` VALUES (10208,20,'2026-06-01 08:21:08.000');
INSERT INTO `watchers` VALUES (10209,16,'2026-06-06 15:02:01.000');
INSERT INTO `watchers` VALUES (10209,21,'2026-06-04 12:13:41.000');
INSERT INTO `watchers` VALUES (10209,23,'2026-06-05 14:41:15.000');
INSERT INTO `watchers` VALUES (10210,17,'2026-06-07 08:43:17.000');
INSERT INTO `watchers` VALUES (10211,2,'2026-06-08 07:25:05.000');
INSERT INTO `watchers` VALUES (10211,9,'2026-06-06 17:15:38.000');
INSERT INTO `watchers` VALUES (10211,10,'2026-06-11 09:39:39.000');
INSERT INTO `watchers` VALUES (10211,11,'2026-06-08 16:11:43.000');
INSERT INTO `watchers` VALUES (10212,2,'2026-06-08 17:40:16.000');
INSERT INTO `watchers` VALUES (10213,7,'2026-06-11 10:30:16.000');
INSERT INTO `watchers` VALUES (10213,11,'2026-06-12 15:28:19.000');
INSERT INTO `watchers` VALUES (10214,2,'2026-06-13 14:59:30.000');
INSERT INTO `watchers` VALUES (10214,7,'2026-06-15 14:57:16.000');
INSERT INTO `watchers` VALUES (10214,16,'2026-06-10 16:18:16.000');
INSERT INTO `watchers` VALUES (10215,13,'2026-06-12 15:47:10.000');
INSERT INTO `watchers` VALUES (10215,15,'2026-06-15 08:01:58.000');
INSERT INTO `watchers` VALUES (10215,19,'2026-06-17 08:30:34.000');
INSERT INTO `watchers` VALUES (10216,5,'2026-06-17 15:17:58.000');
INSERT INTO `watchers` VALUES (10216,12,'2026-06-15 11:10:37.000');
INSERT INTO `watchers` VALUES (10216,13,'2026-06-17 12:10:27.000');
INSERT INTO `watchers` VALUES (10216,23,'2026-06-19 09:25:40.000');
INSERT INTO `watchers` VALUES (10217,6,'2026-06-19 10:55:31.000');
INSERT INTO `watchers` VALUES (10217,17,'2026-06-15 10:45:11.000');
INSERT INTO `watchers` VALUES (10217,20,'2026-06-17 10:37:16.000');
INSERT INTO `watchers` VALUES (10218,6,'2026-06-20 06:18:59.000');
INSERT INTO `watchers` VALUES (10218,7,'2026-06-18 17:36:16.000');
INSERT INTO `watchers` VALUES (10218,17,'2026-06-18 12:39:08.000');
INSERT INTO `watchers` VALUES (10219,15,'2026-06-20 07:01:52.000');
INSERT INTO `watchers` VALUES (10219,17,'2026-06-19 07:08:36.000');
INSERT INTO `watchers` VALUES (10219,21,'2026-06-19 08:05:54.000');
INSERT INTO `watchers` VALUES (10220,7,'2026-06-20 08:44:09.000');
INSERT INTO `watchers` VALUES (10220,11,'2026-06-18 14:07:14.000');
INSERT INTO `watchers` VALUES (10220,14,'2026-06-18 13:01:43.000');
INSERT INTO `watchers` VALUES (10221,2,'2026-06-18 14:26:49.000');
INSERT INTO `watchers` VALUES (10221,3,'2026-06-22 13:28:06.000');
INSERT INTO `watchers` VALUES (10221,10,'2026-06-19 09:23:51.000');
INSERT INTO `watchers` VALUES (10222,4,'2026-06-23 09:57:45.000');
INSERT INTO `watchers` VALUES (10222,17,'2026-06-22 17:48:12.000');
INSERT INTO `watchers` VALUES (10222,20,'2026-06-23 15:55:17.000');
INSERT INTO `watchers` VALUES (10223,2,'2026-06-20 08:37:44.000');
INSERT INTO `watchers` VALUES (10223,7,'2026-06-22 10:56:12.000');
INSERT INTO `watchers` VALUES (10223,9,'2026-06-21 10:37:41.000');
INSERT INTO `watchers` VALUES (10223,21,'2026-06-20 14:50:58.000');
INSERT INTO `watchers` VALUES (10224,9,'2026-06-25 06:39:51.000');
INSERT INTO `watchers` VALUES (10224,15,'2026-06-24 17:37:31.000');
INSERT INTO `watchers` VALUES (10224,23,'2026-06-24 08:00:40.000');
INSERT INTO `watchers` VALUES (10225,8,'2026-06-24 10:13:04.000');
INSERT INTO `watchers` VALUES (10225,14,'2026-06-25 12:37:22.000');
INSERT INTO `watchers` VALUES (10225,15,'2026-06-21 11:36:52.000');
INSERT INTO `watchers` VALUES (10225,21,'2026-06-21 14:20:22.000');
INSERT INTO `watchers` VALUES (10226,12,'2026-06-24 10:30:13.000');
INSERT INTO `watchers` VALUES (10226,15,'2026-06-25 17:27:19.000');
INSERT INTO `watchers` VALUES (10227,7,'2026-06-22 14:57:28.000');
INSERT INTO `watchers` VALUES (10227,12,'2026-06-26 06:37:10.000');
INSERT INTO `watchers` VALUES (10227,21,'2026-06-22 10:57:14.000');
INSERT INTO `watchers` VALUES (10228,9,'2026-06-27 08:58:55.000');
INSERT INTO `watchers` VALUES (10228,12,'2026-06-23 10:04:57.000');
INSERT INTO `watchers` VALUES (10229,5,'2026-07-02 06:24:48.000');
INSERT INTO `watchers` VALUES (10229,9,'2026-07-05 15:00:17.000');
INSERT INTO `watchers` VALUES (10229,11,'2026-07-06 12:51:32.000');
INSERT INTO `watchers` VALUES (10229,12,'2026-07-01 16:33:48.000');
INSERT INTO `watchers` VALUES (10230,11,'2026-07-06 07:50:53.000');
INSERT INTO `watchers` VALUES (10230,17,'2026-07-02 15:07:52.000');
INSERT INTO `watchers` VALUES (10231,7,'2026-07-07 14:37:56.000');
INSERT INTO `watchers` VALUES (10231,10,'2026-07-03 16:45:08.000');
INSERT INTO `watchers` VALUES (10231,19,'2026-07-06 13:33:46.000');
INSERT INTO `watchers` VALUES (10232,3,'2026-07-06 12:38:53.000');
INSERT INTO `watchers` VALUES (10232,9,'2026-07-07 16:18:19.000');
INSERT INTO `watchers` VALUES (10232,22,'2026-07-07 14:25:19.000');
INSERT INTO `watchers` VALUES (10233,12,'2026-07-09 08:47:00.000');
INSERT INTO `watchers` VALUES (10233,14,'2026-07-07 17:36:34.000');
INSERT INTO `watchers` VALUES (10233,21,'2026-07-05 16:32:39.000');
INSERT INTO `watchers` VALUES (10234,9,'2026-07-07 10:26:53.000');
INSERT INTO `watchers` VALUES (10234,10,'2026-07-10 06:12:14.000');
INSERT INTO `watchers` VALUES (10234,16,'2026-07-08 14:30:48.000');
INSERT INTO `watchers` VALUES (10235,2,'2026-07-11 09:30:14.000');
INSERT INTO `watchers` VALUES (10235,9,'2026-07-08 10:59:04.000');
INSERT INTO `watchers` VALUES (10235,12,'2026-07-09 12:06:53.000');
INSERT INTO `watchers` VALUES (10236,3,'2026-07-13 15:51:29.000');
INSERT INTO `watchers` VALUES (10236,15,'2026-07-11 07:20:45.000');
INSERT INTO `watchers` VALUES (10236,17,'2026-07-10 15:51:55.000');
INSERT INTO `watchers` VALUES (10237,2,'2026-07-14 06:31:51.000');
INSERT INTO `watchers` VALUES (10237,3,'2026-07-11 09:40:59.000');
INSERT INTO `watchers` VALUES (10237,7,'2026-07-14 07:34:10.000');
INSERT INTO `watchers` VALUES (10238,7,'2026-07-13 16:01:07.000');
INSERT INTO `watchers` VALUES (10238,15,'2026-07-14 09:38:18.000');
INSERT INTO `watchers` VALUES (10239,10,'2026-07-15 12:33:10.000');
INSERT INTO `watchers` VALUES (10239,16,'2026-07-13 09:38:00.000');
INSERT INTO `watchers` VALUES (10240,4,'2026-07-15 15:49:54.000');
INSERT INTO `watchers` VALUES (10240,6,'2026-07-18 17:23:03.000');
INSERT INTO `watchers` VALUES (10240,18,'2026-07-13 15:54:14.000');
INSERT INTO `watchers` VALUES (10240,21,'2026-07-18 17:38:08.000');
INSERT INTO `watchers` VALUES (10241,5,'2026-07-18 09:52:46.000');
INSERT INTO `watchers` VALUES (10241,8,'2026-07-15 09:58:26.000');
INSERT INTO `watchers` VALUES (10241,14,'2026-07-16 14:33:33.000');
INSERT INTO `watchers` VALUES (10241,19,'2026-07-15 08:07:50.000');
INSERT INTO `watchers` VALUES (10242,6,'2026-07-14 17:00:31.000');
INSERT INTO `watchers` VALUES (10242,10,'2026-07-16 11:49:18.000');
INSERT INTO `watchers` VALUES (10243,8,'2026-07-19 13:58:20.000');
INSERT INTO `watchers` VALUES (10243,10,'2026-07-16 14:17:53.000');
INSERT INTO `watchers` VALUES (10244,7,'2026-07-21 16:44:30.000');
INSERT INTO `watchers` VALUES (10244,13,'2026-07-18 11:21:50.000');
INSERT INTO `watchers` VALUES (10245,2,'2026-07-22 14:54:15.000');
INSERT INTO `watchers` VALUES (10245,10,'2026-07-18 16:11:51.000');
INSERT INTO `watchers` VALUES (10246,2,'2026-07-19 15:42:51.000');
INSERT INTO `watchers` VALUES (10246,5,'2026-07-24 10:58:56.000');
INSERT INTO `watchers` VALUES (10246,15,'2026-07-20 06:02:27.000');
INSERT INTO `watchers` VALUES (10246,22,'2026-07-19 11:54:29.000');
INSERT INTO `watchers` VALUES (10247,1,'2026-07-19 17:30:05.000');
INSERT INTO `watchers` VALUES (10247,8,'2026-07-23 06:55:37.000');
INSERT INTO `watchers` VALUES (10247,9,'2026-07-24 15:28:15.000');
INSERT INTO `watchers` VALUES (10247,14,'2026-07-24 10:10:40.000');
INSERT INTO `watchers` VALUES (10248,21,'2026-07-28 14:41:40.000');
INSERT INTO `watchers` VALUES (10248,22,'2026-07-26 08:06:23.000');
INSERT INTO `watchers` VALUES (10248,23,'2026-07-24 17:37:36.000');
INSERT INTO `watchers` VALUES (10249,1,'2026-07-25 16:11:50.000');
INSERT INTO `watchers` VALUES (10249,2,'2026-07-26 14:02:47.000');
INSERT INTO `watchers` VALUES (10249,13,'2026-07-29 07:36:50.000');
INSERT INTO `watchers` VALUES (10249,17,'2026-07-28 16:48:17.000');
INSERT INTO `watchers` VALUES (10250,2,'2026-07-30 06:00:33.000');
INSERT INTO `watchers` VALUES (10250,3,'2026-07-31 15:52:52.000');
INSERT INTO `watchers` VALUES (10250,19,'2026-07-31 10:32:18.000');
INSERT INTO `watchers` VALUES (10250,23,'2026-07-30 14:57:12.000');
INSERT INTO `watchers` VALUES (10251,2,'2026-07-28 13:46:07.000');
INSERT INTO `watchers` VALUES (10251,20,'2026-07-31 08:42:33.000');
INSERT INTO `watchers` VALUES (10252,9,'2026-07-30 16:59:29.000');
INSERT INTO `watchers` VALUES (10252,13,'2026-08-04 17:36:30.000');
INSERT INTO `watchers` VALUES (10253,4,'2026-08-02 17:43:45.000');
INSERT INTO `watchers` VALUES (10253,11,'2026-08-01 15:38:45.000');
INSERT INTO `watchers` VALUES (10253,20,'2026-08-02 14:31:33.000');
INSERT INTO `watchers` VALUES (10254,1,'2026-08-02 10:51:50.000');
INSERT INTO `watchers` VALUES (10254,5,'2026-08-02 09:18:42.000');
INSERT INTO `watchers` VALUES (10254,12,'2026-08-04 13:46:31.000');
INSERT INTO `watchers` VALUES (10254,16,'2026-08-02 07:24:37.000');
INSERT INTO `watchers` VALUES (10255,8,'2026-08-04 15:00:03.000');
INSERT INTO `watchers` VALUES (10255,14,'2026-08-06 07:08:00.000');
INSERT INTO `watchers` VALUES (10256,5,'2026-08-08 08:25:58.000');
INSERT INTO `watchers` VALUES (10256,15,'2026-08-08 11:41:12.000');
INSERT INTO `watchers` VALUES (10256,22,'2026-08-06 10:23:01.000');
INSERT INTO `watchers` VALUES (10257,10,'2026-08-07 15:31:46.000');
INSERT INTO `watchers` VALUES (10257,13,'2026-08-06 14:05:06.000');
INSERT INTO `watchers` VALUES (10257,16,'2026-08-06 16:26:23.000');
INSERT INTO `watchers` VALUES (10258,9,'2026-08-06 09:11:35.000');
INSERT INTO `watchers` VALUES (10258,13,'2026-08-11 11:01:33.000');
INSERT INTO `watchers` VALUES (10258,23,'2026-08-06 10:16:10.000');
INSERT INTO `watchers` VALUES (10259,2,'2026-08-09 14:46:27.000');
INSERT INTO `watchers` VALUES (10259,3,'2026-08-09 07:36:16.000');
INSERT INTO `watchers` VALUES (10259,11,'2026-08-09 07:22:22.000');
INSERT INTO `watchers` VALUES (10260,2,'2026-08-12 13:54:21.000');
INSERT INTO `watchers` VALUES (10260,12,'2026-08-12 07:37:19.000');
INSERT INTO `watchers` VALUES (10260,22,'2026-08-11 17:27:00.000');
INSERT INTO `watchers` VALUES (10261,2,'2026-08-10 06:00:11.000');
INSERT INTO `watchers` VALUES (10261,8,'2026-08-13 13:39:40.000');
INSERT INTO `watchers` VALUES (10261,19,'2026-08-09 07:04:53.000');
INSERT INTO `watchers` VALUES (10261,23,'2026-08-08 16:52:25.000');
INSERT INTO `watchers` VALUES (10262,3,'2026-08-14 11:40:47.000');
INSERT INTO `watchers` VALUES (10262,22,'2026-08-11 17:19:05.000');
INSERT INTO `watchers` VALUES (10263,6,'2026-08-15 10:14:26.000');
INSERT INTO `watchers` VALUES (10263,11,'2026-08-18 13:59:05.000');
INSERT INTO `watchers` VALUES (10263,13,'2026-08-15 15:19:21.000');
INSERT INTO `watchers` VALUES (10263,18,'2026-08-18 15:10:37.000');
INSERT INTO `watchers` VALUES (10264,2,'2026-08-15 16:47:03.000');
INSERT INTO `watchers` VALUES (10264,17,'2026-08-18 15:09:16.000');
INSERT INTO `watchers` VALUES (10265,9,'2026-08-21 09:19:49.000');
INSERT INTO `watchers` VALUES (10265,12,'2026-08-23 17:13:00.000');
INSERT INTO `watchers` VALUES (10265,17,'2026-08-24 12:56:24.000');
INSERT INTO `watchers` VALUES (10266,5,'2026-08-27 09:59:19.000');
INSERT INTO `watchers` VALUES (10266,16,'2026-08-25 11:37:18.000');
INSERT INTO `watchers` VALUES (10266,17,'2026-08-25 14:36:08.000');
INSERT INTO `watchers` VALUES (10267,15,'2026-08-22 16:41:28.000');
INSERT INTO `watchers` VALUES (10267,21,'2026-08-23 11:49:35.000');
INSERT INTO `watchers` VALUES (10268,10,'2026-08-27 08:49:27.000');
INSERT INTO `watchers` VALUES (10269,9,'2026-08-31 16:09:00.000');
INSERT INTO `watchers` VALUES (10269,23,'2026-08-30 06:01:06.000');
INSERT INTO `watchers` VALUES (10270,10,'2026-08-28 15:01:16.000');
INSERT INTO `watchers` VALUES (10270,14,'2026-09-01 06:52:00.000');
INSERT INTO `watchers` VALUES (10270,16,'2026-08-27 11:21:39.000');
INSERT INTO `watchers` VALUES (10271,2,'2026-08-30 12:26:22.000');
INSERT INTO `watchers` VALUES (10271,9,'2026-08-31 14:53:14.000');
INSERT INTO `watchers` VALUES (10271,13,'2026-08-30 13:34:57.000');
INSERT INTO `watchers` VALUES (10272,3,'2026-08-31 16:16:53.000');
INSERT INTO `watchers` VALUES (10272,7,'2026-08-31 12:18:29.000');
INSERT INTO `watchers` VALUES (10272,16,'2026-08-29 08:51:25.000');
INSERT INTO `watchers` VALUES (10273,4,'2026-09-01 16:39:51.000');
INSERT INTO `watchers` VALUES (10273,6,'2026-09-02 14:18:33.000');
INSERT INTO `watchers` VALUES (10273,17,'2026-08-31 13:15:56.000');
INSERT INTO `watchers` VALUES (10273,18,'2026-09-01 10:45:21.000');
INSERT INTO `watchers` VALUES (10274,15,'2026-08-29 14:25:05.000');
INSERT INTO `watchers` VALUES (10274,19,'2026-08-29 11:44:36.000');
INSERT INTO `watchers` VALUES (10274,21,'2026-09-01 15:40:22.000');
INSERT INTO `watchers` VALUES (10275,7,'2026-09-02 12:51:05.000');
INSERT INTO `watchers` VALUES (10275,23,'2026-09-03 07:33:33.000');
INSERT INTO `watchers` VALUES (10276,1,'2026-08-30 16:20:28.000');
INSERT INTO `watchers` VALUES (10276,6,'2026-09-01 14:19:22.000');
INSERT INTO `watchers` VALUES (10276,11,'2026-09-01 07:27:13.000');
INSERT INTO `watchers` VALUES (10277,2,'2026-09-03 06:49:34.000');
INSERT INTO `watchers` VALUES (10277,11,'2026-09-01 09:53:28.000');
INSERT INTO `watchers` VALUES (10277,21,'2026-09-05 06:52:49.000');
INSERT INTO `watchers` VALUES (10277,23,'2026-09-02 13:53:11.000');
INSERT INTO `watchers` VALUES (10278,1,'2026-09-06 08:03:31.000');
INSERT INTO `watchers` VALUES (10278,8,'2026-09-02 09:59:34.000');
INSERT INTO `watchers` VALUES (10279,2,'2026-09-03 17:34:08.000');
INSERT INTO `watchers` VALUES (10279,11,'2026-09-02 17:12:51.000');
INSERT INTO `watchers` VALUES (10280,5,'2026-09-06 07:20:01.000');
INSERT INTO `watchers` VALUES (10280,6,'2026-09-08 13:09:33.000');
INSERT INTO `watchers` VALUES (10280,17,'2026-09-08 07:33:27.000');
INSERT INTO `watchers` VALUES (10281,11,'2026-09-07 10:10:21.000');
INSERT INTO `watchers` VALUES (10281,16,'2026-09-08 08:10:01.000');
INSERT INTO `watchers` VALUES (10281,19,'2026-09-07 15:05:22.000');
INSERT INTO `watchers` VALUES (10282,2,'2026-09-11 10:01:33.000');
INSERT INTO `watchers` VALUES (10282,11,'2026-09-08 17:57:05.000');
INSERT INTO `watchers` VALUES (10282,15,'2026-09-10 15:36:33.000');
INSERT INTO `watchers` VALUES (10282,18,'2026-09-08 17:55:36.000');
INSERT INTO `watchers` VALUES (10283,9,'2026-09-10 14:48:00.000');
INSERT INTO `watchers` VALUES (10283,11,'2026-09-14 12:30:23.000');
INSERT INTO `watchers` VALUES (10283,21,'2026-09-11 10:05:22.000');
INSERT INTO `watchers` VALUES (10284,12,'2026-09-12 16:18:24.000');
INSERT INTO `watchers` VALUES (10284,15,'2026-09-12 15:37:31.000');
INSERT INTO `watchers` VALUES (10284,16,'2026-09-12 12:24:10.000');
INSERT INTO `watchers` VALUES (10284,17,'2026-09-15 06:15:43.000');
INSERT INTO `watchers` VALUES (10285,4,'2026-09-14 17:05:53.000');
INSERT INTO `watchers` VALUES (10285,15,'2026-09-13 07:01:43.000');
INSERT INTO `watchers` VALUES (10285,23,'2026-09-12 17:40:59.000');
INSERT INTO `watchers` VALUES (10286,12,'2026-09-13 17:43:15.000');
INSERT INTO `watchers` VALUES (10286,13,'2026-09-15 12:31:09.000');
INSERT INTO `watchers` VALUES (10286,16,'2026-09-13 06:35:41.000');
INSERT INTO `watchers` VALUES (10286,17,'2026-09-16 09:42:56.000');
INSERT INTO `watchers` VALUES (10287,6,'2026-09-11 17:16:39.000');
INSERT INTO `watchers` VALUES (10287,22,'2026-09-15 12:05:08.000');
INSERT INTO `watchers` VALUES (10287,23,'2026-09-12 08:33:48.000');
INSERT INTO `watchers` VALUES (10288,3,'2026-09-15 13:13:01.000');
INSERT INTO `watchers` VALUES (10288,9,'2026-09-16 06:30:10.000');
INSERT INTO `watchers` VALUES (10288,19,'2026-09-18 07:53:29.000');
INSERT INTO `watchers` VALUES (10289,2,'2026-09-15 17:57:39.000');
INSERT INTO `watchers` VALUES (10289,20,'2026-09-17 14:55:32.000');
INSERT INTO `watchers` VALUES (10290,4,'2026-09-17 17:01:24.000');
INSERT INTO `watchers` VALUES (10290,17,'2026-09-17 17:37:40.000');
INSERT INTO `watchers` VALUES (10291,4,'2026-09-19 15:56:51.000');
INSERT INTO `watchers` VALUES (10291,11,'2026-09-22 13:10:30.000');
INSERT INTO `watchers` VALUES (10292,5,'2026-09-24 13:00:41.000');
INSERT INTO `watchers` VALUES (10292,13,'2026-09-25 13:53:18.000');
INSERT INTO `watchers` VALUES (10292,14,'2026-09-25 13:52:29.000');
INSERT INTO `watchers` VALUES (10292,15,'2026-09-25 06:16:58.000');
INSERT INTO `watchers` VALUES (10293,21,'2026-09-21 14:23:51.000');
INSERT INTO `watchers` VALUES (10293,23,'2026-09-23 15:52:42.000');
INSERT INTO `watchers` VALUES (10294,12,'2026-09-23 16:52:25.000');
INSERT INTO `watchers` VALUES (10295,2,'2026-09-30 16:48:07.000');
INSERT INTO `watchers` VALUES (10295,6,'2026-09-30 14:26:23.000');
INSERT INTO `watchers` VALUES (10295,13,'2026-09-30 11:49:07.000');
INSERT INTO `watchers` VALUES (10295,16,'2026-09-26 17:39:58.000');
INSERT INTO `watchers` VALUES (10296,6,'2026-09-26 13:54:03.000');
INSERT INTO `watchers` VALUES (10296,15,'2026-09-28 12:43:58.000');
INSERT INTO `watchers` VALUES (10296,22,'2026-09-25 12:54:04.000');
INSERT INTO `watchers` VALUES (10296,23,'2026-09-27 06:49:31.000');
INSERT INTO `watchers` VALUES (10297,5,'2026-09-27 13:01:58.000');
INSERT INTO `watchers` VALUES (10297,16,'2026-09-30 14:10:50.000');
INSERT INTO `watchers` VALUES (10298,11,'2026-10-01 16:00:00.000');
INSERT INTO `watchers` VALUES (10298,12,'2026-10-01 16:00:00.000');
INSERT INTO `watchers` VALUES (10298,17,'2026-09-30 10:06:28.000');
INSERT INTO `watchers` VALUES (10299,1,'2026-10-01 16:00:00.000');
INSERT INTO `watchers` VALUES (10299,9,'2026-10-01 16:00:00.000');
/*!40000 ALTER TABLE `watchers` ENABLE KEYS */;
DROP TABLE IF EXISTS `webhook_deliveries`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `webhook_deliveries` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `webhook_id` bigint(20) NOT NULL,
  `event` varchar(60) NOT NULL,
  `url` text NOT NULL,
  `payload` mediumtext NOT NULL,
  `status` varchar(10) NOT NULL DEFAULT 'pending',
  `attempts` int(11) NOT NULL DEFAULT 0,
  `next_attempt_at` datetime(3) NOT NULL,
  `claim` char(32) DEFAULT NULL,
  `status_code` int(11) DEFAULT NULL,
  `duration_ms` int(11) DEFAULT NULL,
  `error` text DEFAULT NULL,
  `created_at` datetime(3) NOT NULL,
  `attempted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_wd_due` (`status`,`next_attempt_at`),
  KEY `ix_wd_hook` (`webhook_id`,`id`),
  KEY `ix_wd_claim` (`claim`),
  CONSTRAINT `fk_wd_webhook` FOREIGN KEY (`webhook_id`) REFERENCES `webhooks` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `webhook_deliveries` DISABLE KEYS */;
/*!40000 ALTER TABLE `webhook_deliveries` ENABLE KEYS */;
DROP TABLE IF EXISTS `webhooks`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `webhooks` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL,
  `url` text NOT NULL,
  `events` varchar(1000) NOT NULL,
  `jql` text DEFAULT NULL,
  `secret` text DEFAULT NULL,
  `enabled` tinyint(1) NOT NULL DEFAULT 1,
  `exclude_body` tinyint(1) NOT NULL DEFAULT 0,
  `created_by` bigint(20) DEFAULT NULL,
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_webhooks_user` (`created_by`),
  CONSTRAINT `fk_webhooks_user` FOREIGN KEY (`created_by`) REFERENCES `users` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `webhooks` DISABLE KEYS */;
/*!40000 ALTER TABLE `webhooks` ENABLE KEYS */;
DROP TABLE IF EXISTS `workflow_issue_types`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `workflow_issue_types` (
  `project_id` bigint(20) NOT NULL,
  `issue_type_id` varchar(60) NOT NULL,
  `workflow_id` bigint(20) NOT NULL,
  PRIMARY KEY (`project_id`,`issue_type_id`),
  KEY `fk_wit_type` (`issue_type_id`),
  KEY `fk_wit_workflow` (`workflow_id`),
  CONSTRAINT `fk_wit_project` FOREIGN KEY (`project_id`) REFERENCES `projects` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_wit_type` FOREIGN KEY (`issue_type_id`) REFERENCES `issue_types` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_wit_workflow` FOREIGN KEY (`workflow_id`) REFERENCES `workflows` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `workflow_issue_types` DISABLE KEYS */;
/*!40000 ALTER TABLE `workflow_issue_types` ENABLE KEYS */;
DROP TABLE IF EXISTS `workflow_statuses`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `workflow_statuses` (
  `workflow_id` bigint(20) NOT NULL,
  `status_id` varchar(60) NOT NULL,
  `position` int(11) NOT NULL DEFAULT 0,
  PRIMARY KEY (`workflow_id`,`status_id`),
  KEY `fk_ws_status` (`status_id`),
  CONSTRAINT `fk_ws_status` FOREIGN KEY (`status_id`) REFERENCES `statuses` (`id`),
  CONSTRAINT `fk_ws_workflow` FOREIGN KEY (`workflow_id`) REFERENCES `workflows` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `workflow_statuses` DISABLE KEYS */;
INSERT INTO `workflow_statuses` VALUES (1,'10000',0);
INSERT INTO `workflow_statuses` VALUES (1,'10001',3);
INSERT INTO `workflow_statuses` VALUES (1,'10002',2);
INSERT INTO `workflow_statuses` VALUES (1,'3',1);
INSERT INTO `workflow_statuses` VALUES (2,'10000',0);
INSERT INTO `workflow_statuses` VALUES (2,'10001',3);
INSERT INTO `workflow_statuses` VALUES (2,'10002',2);
INSERT INTO `workflow_statuses` VALUES (2,'3',1);
INSERT INTO `workflow_statuses` VALUES (3,'1',0);
INSERT INTO `workflow_statuses` VALUES (3,'3',1);
INSERT INTO `workflow_statuses` VALUES (3,'4',3);
INSERT INTO `workflow_statuses` VALUES (3,'5',2);
INSERT INTO `workflow_statuses` VALUES (3,'6',4);
INSERT INTO `workflow_statuses` VALUES (4,'1',0);
INSERT INTO `workflow_statuses` VALUES (4,'3',1);
INSERT INTO `workflow_statuses` VALUES (4,'4',3);
INSERT INTO `workflow_statuses` VALUES (4,'5',2);
INSERT INTO `workflow_statuses` VALUES (4,'6',4);
/*!40000 ALTER TABLE `workflow_statuses` ENABLE KEYS */;
DROP TABLE IF EXISTS `workflow_transitions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `workflow_transitions` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `workflow_id` bigint(20) NOT NULL,
  `name` varchar(255) NOT NULL,
  `from_status_id` varchar(60) DEFAULT NULL,
  `to_status_id` varchar(60) NOT NULL,
  `position` int(11) NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `ix_wt_workflow` (`workflow_id`),
  KEY `fk_wt_from` (`from_status_id`),
  KEY `fk_wt_to` (`to_status_id`),
  CONSTRAINT `fk_wt_from` FOREIGN KEY (`from_status_id`) REFERENCES `statuses` (`id`),
  CONSTRAINT `fk_wt_to` FOREIGN KEY (`to_status_id`) REFERENCES `statuses` (`id`),
  CONSTRAINT `fk_wt_workflow` FOREIGN KEY (`workflow_id`) REFERENCES `workflows` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=39 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `workflow_transitions` DISABLE KEYS */;
INSERT INTO `workflow_transitions` VALUES (1,1,'Démarrer','10000','3',0);
INSERT INTO `workflow_transitions` VALUES (2,1,'Demander une revue','3','10002',1);
INSERT INTO `workflow_transitions` VALUES (3,1,'Remettre à faire','3','10000',2);
INSERT INTO `workflow_transitions` VALUES (4,1,'Valider','10002','10001',3);
INSERT INTO `workflow_transitions` VALUES (5,1,'Renvoyer en cours','10002','3',4);
INSERT INTO `workflow_transitions` VALUES (6,1,'Rouvrir','10001','10000',5);
INSERT INTO `workflow_transitions` VALUES (7,1,'Clore directement',NULL,'10001',6);
INSERT INTO `workflow_transitions` VALUES (8,2,'Démarrer','10000','3',0);
INSERT INTO `workflow_transitions` VALUES (9,2,'Demander une revue','3','10002',1);
INSERT INTO `workflow_transitions` VALUES (10,2,'Remettre à faire','3','10000',2);
INSERT INTO `workflow_transitions` VALUES (11,2,'Valider','10002','10001',3);
INSERT INTO `workflow_transitions` VALUES (12,2,'Renvoyer en cours','10002','3',4);
INSERT INTO `workflow_transitions` VALUES (13,2,'Rouvrir','10001','10000',5);
INSERT INTO `workflow_transitions` VALUES (14,2,'Clore directement',NULL,'10001',6);
INSERT INTO `workflow_transitions` VALUES (15,3,'Start progress','1','3',0);
INSERT INTO `workflow_transitions` VALUES (16,3,'Resolve','1','5',1);
INSERT INTO `workflow_transitions` VALUES (17,3,'Close','1','6',2);
INSERT INTO `workflow_transitions` VALUES (18,3,'Stop progress','3','1',3);
INSERT INTO `workflow_transitions` VALUES (19,3,'Resolve','3','5',4);
INSERT INTO `workflow_transitions` VALUES (20,3,'Close','3','6',5);
INSERT INTO `workflow_transitions` VALUES (21,3,'Close','5','6',6);
INSERT INTO `workflow_transitions` VALUES (22,3,'Reopen','5','4',7);
INSERT INTO `workflow_transitions` VALUES (23,3,'Start progress','4','3',8);
INSERT INTO `workflow_transitions` VALUES (24,3,'Resolve','4','5',9);
INSERT INTO `workflow_transitions` VALUES (25,3,'Close','4','6',10);
INSERT INTO `workflow_transitions` VALUES (26,3,'Reopen','6','4',11);
INSERT INTO `workflow_transitions` VALUES (27,4,'Start progress','1','3',0);
INSERT INTO `workflow_transitions` VALUES (28,4,'Resolve','1','5',1);
INSERT INTO `workflow_transitions` VALUES (29,4,'Close','1','6',2);
INSERT INTO `workflow_transitions` VALUES (30,4,'Stop progress','3','1',3);
INSERT INTO `workflow_transitions` VALUES (31,4,'Resolve','3','5',4);
INSERT INTO `workflow_transitions` VALUES (32,4,'Close','3','6',5);
INSERT INTO `workflow_transitions` VALUES (33,4,'Close','5','6',6);
INSERT INTO `workflow_transitions` VALUES (34,4,'Reopen','5','4',7);
INSERT INTO `workflow_transitions` VALUES (35,4,'Start progress','4','3',8);
INSERT INTO `workflow_transitions` VALUES (36,4,'Resolve','4','5',9);
INSERT INTO `workflow_transitions` VALUES (37,4,'Close','4','6',10);
INSERT INTO `workflow_transitions` VALUES (38,4,'Reopen','6','4',11);
/*!40000 ALTER TABLE `workflow_transitions` ENABLE KEYS */;
DROP TABLE IF EXISTS `workflows`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `workflows` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `project_id` bigint(20) NOT NULL,
  `name` varchar(255) NOT NULL,
  `is_default` tinyint(1) NOT NULL DEFAULT 0,
  `initial_status_id` varchar(60) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_workflows_project` (`project_id`),
  KEY `fk_workflows_initial` (`initial_status_id`),
  CONSTRAINT `fk_workflows_initial` FOREIGN KEY (`initial_status_id`) REFERENCES `statuses` (`id`),
  CONSTRAINT `fk_workflows_project` FOREIGN KEY (`project_id`) REFERENCES `projects` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `workflows` DISABLE KEYS */;
INSERT INTO `workflows` VALUES (1,10000,'Logiciel',1,'10000');
INSERT INTO `workflows` VALUES (2,10001,'Logiciel',1,'10000');
INSERT INTO `workflows` VALUES (3,10002,'jira',1,'1');
INSERT INTO `workflows` VALUES (4,10003,'jira',1,'1');
/*!40000 ALTER TABLE `workflows` ENABLE KEYS */;
DROP TABLE IF EXISTS `worklogs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `worklogs` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `issue_id` bigint(20) NOT NULL,
  `author_id` bigint(20) DEFAULT NULL,
  `body` longtext DEFAULT NULL,
  `started_at` datetime(3) NOT NULL,
  `seconds` bigint(20) NOT NULL,
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  `updated_by_id` bigint(20) DEFAULT NULL,
  `visibility_group` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin DEFAULT NULL,
  `visibility_role_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_worklogs_issue` (`issue_id`),
  KEY `fk_worklogs_author` (`author_id`),
  KEY `fk_worklogs_updated_by` (`updated_by_id`),
  KEY `fk_worklogs_role` (`visibility_role_id`),
  CONSTRAINT `fk_worklogs_author` FOREIGN KEY (`author_id`) REFERENCES `users` (`id`) ON DELETE SET NULL,
  CONSTRAINT `fk_worklogs_issue` FOREIGN KEY (`issue_id`) REFERENCES `issues` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_worklogs_role` FOREIGN KEY (`visibility_role_id`) REFERENCES `project_roles` (`id`),
  CONSTRAINT `fk_worklogs_updated_by` FOREIGN KEY (`updated_by_id`) REFERENCES `users` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB AUTO_INCREMENT=10158 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

/*!40000 ALTER TABLE `worklogs` DISABLE KEYS */;
INSERT INTO `worklogs` VALUES (10000,10008,5,'Tests et recette','2025-09-19 10:36:16.000',27900,'2025-09-19 13:37:16.000','2025-09-19 13:37:16.000',5,'developpeurs',NULL);
INSERT INTO `worklogs` VALUES (10001,10008,5,NULL,'2025-09-28 08:15:40.000',10800,'2025-09-28 09:28:19.000','2025-09-28 09:28:19.000',5,NULL,NULL);
INSERT INTO `worklogs` VALUES (10002,10008,5,'Tests et recette','2025-09-29 07:46:16.000',27000,'2025-09-29 17:31:07.000','2025-09-29 17:31:07.000',5,NULL,NULL);
INSERT INTO `worklogs` VALUES (10003,10013,10,'Point avec le client','2025-09-28 10:47:06.000',28800,'2025-09-28 17:11:05.000','2025-09-28 17:11:05.000',10,NULL,NULL);
INSERT INTO `worklogs` VALUES (10004,10013,10,'Revue de code','2025-10-06 07:45:53.000',8100,'2025-10-06 09:41:16.000','2025-10-06 09:41:16.000',10,NULL,NULL);
INSERT INTO `worklogs` VALUES (10005,10013,10,'Analyse','2025-10-07 12:16:45.000',8100,'2025-10-07 16:56:47.000','2025-10-07 16:56:47.000',10,NULL,NULL);
INSERT INTO `worklogs` VALUES (10006,10013,10,'Analyse','2025-10-10 17:27:42.000',20700,'2025-10-10 17:32:26.000','2025-10-10 17:32:26.000',10,NULL,NULL);
INSERT INTO `worklogs` VALUES (10007,10015,11,'Analyse','2025-10-08 13:56:07.000',2700,'2025-10-08 16:22:19.000','2025-10-08 16:22:19.000',11,NULL,NULL);
INSERT INTO `worklogs` VALUES (10008,10015,11,'Point avec le client','2025-10-15 09:41:05.000',7200,'2025-10-15 14:52:20.000','2025-10-15 14:52:20.000',11,NULL,NULL);
INSERT INTO `worklogs` VALUES (10009,10015,11,NULL,'2025-10-19 14:48:05.000',22500,'2025-10-19 15:00:29.000','2025-10-19 15:00:29.000',11,NULL,NULL);
INSERT INTO `worklogs` VALUES (10010,10027,9,'Point avec le client','2025-10-26 13:20:17.000',27900,'2025-10-26 17:10:02.000','2025-10-26 17:10:02.000',9,NULL,NULL);
INSERT INTO `worklogs` VALUES (10011,10027,9,'Revue de code','2025-11-03 12:28:49.000',27000,'2025-11-03 16:50:48.000','2025-11-03 16:50:48.000',9,NULL,NULL);
INSERT INTO `worklogs` VALUES (10012,10027,9,'Revue de code','2025-11-03 14:26:16.000',3600,'2025-11-03 18:55:28.000','2025-11-03 18:55:28.000',9,NULL,NULL);
INSERT INTO `worklogs` VALUES (10013,10027,9,NULL,'2025-11-03 18:42:25.000',14400,'2025-11-03 18:51:16.000','2025-11-03 18:51:16.000',9,NULL,NULL);
INSERT INTO `worklogs` VALUES (10014,10029,11,'Analyse','2025-10-27 18:44:16.000',9900,'2025-10-27 18:58:43.000','2025-10-27 18:58:43.000',11,NULL,NULL);
INSERT INTO `worklogs` VALUES (10015,10029,11,NULL,'2025-11-05 14:10:50.000',7200,'2025-11-05 17:53:37.000','2025-11-05 17:53:37.000',11,NULL,NULL);
INSERT INTO `worklogs` VALUES (10016,10029,11,NULL,'2025-11-05 18:58:35.000',11700,'2025-11-05 18:58:36.000','2025-11-05 18:58:36.000',11,NULL,NULL);
INSERT INTO `worklogs` VALUES (10017,10039,4,NULL,'2025-11-09 14:30:39.000',17100,'2025-11-09 17:12:17.000','2025-11-09 17:12:17.000',4,NULL,NULL);
INSERT INTO `worklogs` VALUES (10018,10039,4,'Point avec le client','2025-11-15 08:41:45.000',9900,'2025-11-15 14:01:24.000','2025-11-15 14:01:24.000',4,NULL,NULL);
INSERT INTO `worklogs` VALUES (10019,10039,4,'Développement','2025-11-25 14:40:22.000',1800,'2025-11-25 15:42:51.000','2025-11-25 15:42:51.000',4,NULL,NULL);
INSERT INTO `worklogs` VALUES (10020,10039,4,'Développement','2025-12-02 17:16:06.000',22500,'2025-12-02 18:19:05.000','2025-12-02 18:19:05.000',4,NULL,NULL);
INSERT INTO `worklogs` VALUES (10021,10040,13,NULL,'2025-11-11 10:30:30.000',14400,'2025-11-11 11:08:44.000','2025-11-11 11:08:44.000',13,NULL,NULL);
INSERT INTO `worklogs` VALUES (10022,10040,6,'Analyse','2025-11-21 13:42:32.000',14400,'2025-11-21 17:34:33.000','2025-11-21 17:34:33.000',6,NULL,NULL);
INSERT INTO `worklogs` VALUES (10023,10040,3,'Tests et recette','2025-11-26 08:23:18.000',2700,'2025-11-26 11:30:33.000','2025-11-26 11:30:33.000',3,NULL,NULL);
INSERT INTO `worklogs` VALUES (10024,10040,2,NULL,'2025-12-01 12:38:28.000',22500,'2025-12-01 17:16:15.000','2025-12-01 17:16:15.000',2,NULL,NULL);
INSERT INTO `worklogs` VALUES (10025,10043,7,'Point avec le client','2025-11-17 13:27:46.000',18900,'2025-11-17 16:50:05.000','2025-11-17 16:50:05.000',7,NULL,NULL);
INSERT INTO `worklogs` VALUES (10026,10043,7,NULL,'2025-11-25 14:46:31.000',11700,'2025-11-25 16:15:27.000','2025-11-25 16:15:27.000',7,NULL,NULL);
INSERT INTO `worklogs` VALUES (10027,10043,7,'Analyse','2025-11-26 08:58:09.000',13500,'2025-11-26 15:51:51.000','2025-11-26 15:51:51.000',7,NULL,NULL);
INSERT INTO `worklogs` VALUES (10028,10043,7,NULL,'2025-11-29 12:38:13.000',13500,'2025-11-29 12:38:29.000','2025-11-29 12:38:29.000',7,NULL,NULL);
INSERT INTO `worklogs` VALUES (10029,10056,6,'Point avec le client','2025-12-02 18:13:26.000',19800,'2025-12-02 18:15:53.000','2025-12-02 18:15:53.000',6,NULL,NULL);
INSERT INTO `worklogs` VALUES (10030,10056,6,'Déploiement','2025-12-05 08:22:59.000',26100,'2025-12-05 10:08:47.000','2025-12-05 10:08:47.000',6,NULL,NULL);
INSERT INTO `worklogs` VALUES (10031,10056,6,'Point avec le client','2025-12-10 17:30:07.000',15300,'2025-12-10 17:42:54.000','2025-12-10 17:42:54.000',6,NULL,NULL);
INSERT INTO `worklogs` VALUES (10032,10056,6,'Point avec le client','2025-12-19 13:02:39.000',27000,'2025-12-19 15:16:06.000','2025-12-19 15:16:06.000',6,NULL,NULL);
INSERT INTO `worklogs` VALUES (10033,10057,22,'Point avec le client','2025-11-30 15:18:30.000',6300,'2025-11-30 15:28:21.000','2025-11-30 15:28:21.000',22,NULL,NULL);
INSERT INTO `worklogs` VALUES (10034,10062,4,NULL,'2025-12-04 10:12:13.000',15300,'2025-12-04 12:10:18.000','2025-12-04 12:10:18.000',4,NULL,NULL);
INSERT INTO `worklogs` VALUES (10035,10064,16,NULL,'2025-12-04 18:21:48.000',11700,'2025-12-04 18:35:26.000','2025-12-04 18:35:26.000',16,NULL,NULL);
INSERT INTO `worklogs` VALUES (10036,10064,16,'Point avec le client','2025-12-04 18:25:56.000',8100,'2025-12-04 18:26:02.000','2025-12-04 18:26:02.000',16,NULL,NULL);
INSERT INTO `worklogs` VALUES (10037,10064,16,'Revue de code','2025-12-12 14:19:48.000',2700,'2025-12-12 17:51:10.000','2025-12-12 17:51:10.000',16,NULL,NULL);
INSERT INTO `worklogs` VALUES (10038,10064,16,'Point avec le client','2025-12-21 15:14:07.000',7200,'2025-12-21 16:25:15.000','2025-12-21 16:25:15.000',16,NULL,NULL);
INSERT INTO `worklogs` VALUES (10039,10075,3,'Déploiement','2025-12-19 11:46:54.000',18000,'2025-12-19 13:40:38.000','2025-12-19 13:40:38.000',3,NULL,NULL);
INSERT INTO `worklogs` VALUES (10040,10077,3,NULL,'2025-12-25 08:32:19.000',3600,'2025-12-25 10:45:42.000','2025-12-25 10:45:42.000',3,NULL,NULL);
INSERT INTO `worklogs` VALUES (10041,10077,16,'Point avec le client','2025-12-30 15:42:10.000',2700,'2025-12-30 16:49:28.000','2025-12-30 16:49:28.000',16,NULL,NULL);
INSERT INTO `worklogs` VALUES (10042,10078,14,'Tests et recette','2025-12-21 17:10:34.000',9900,'2025-12-21 17:53:02.000','2025-12-21 17:53:02.000',14,NULL,NULL);
INSERT INTO `worklogs` VALUES (10043,10078,14,'Analyse','2025-12-28 11:31:54.000',14400,'2025-12-28 15:49:21.000','2025-12-28 15:49:21.000',14,NULL,NULL);
INSERT INTO `worklogs` VALUES (10044,10078,14,'Déploiement','2025-12-31 10:17:38.000',24300,'2025-12-31 17:29:23.000','2025-12-31 17:29:23.000',14,NULL,NULL);
INSERT INTO `worklogs` VALUES (10045,10081,17,NULL,'2025-12-28 08:06:58.000',18000,'2025-12-28 09:23:18.000','2025-12-28 09:23:18.000',17,NULL,NULL);
INSERT INTO `worklogs` VALUES (10046,10081,17,'Point avec le client','2025-12-28 11:36:11.000',12600,'2025-12-28 12:13:31.000','2025-12-28 12:13:31.000',17,NULL,NULL);
INSERT INTO `worklogs` VALUES (10047,10085,3,'Développement','2026-01-01 13:55:50.000',9000,'2026-01-01 16:30:58.000','2026-01-01 16:30:58.000',3,NULL,NULL);
INSERT INTO `worklogs` VALUES (10048,10085,3,'Tests et recette','2026-01-05 07:47:50.000',28800,'2026-01-05 16:52:32.000','2026-01-05 16:52:32.000',3,NULL,NULL);
INSERT INTO `worklogs` VALUES (10049,10085,3,'Déploiement','2026-01-05 15:58:04.000',26100,'2026-01-05 18:53:37.000','2026-01-05 18:53:37.000',3,NULL,NULL);
INSERT INTO `worklogs` VALUES (10050,10086,21,'Analyse','2026-01-09 08:02:49.000',11700,'2026-01-09 11:10:40.000','2026-01-09 11:10:40.000',21,NULL,NULL);
INSERT INTO `worklogs` VALUES (10051,10086,21,'Développement','2026-01-10 09:06:32.000',8100,'2026-01-10 16:50:55.000','2026-01-10 16:50:55.000',21,NULL,NULL);
INSERT INTO `worklogs` VALUES (10052,10086,21,NULL,'2026-01-17 11:22:28.000',25200,'2026-01-17 17:31:51.000','2026-01-17 17:31:51.000',21,NULL,NULL);
INSERT INTO `worklogs` VALUES (10053,10086,21,'Point avec le client','2026-01-27 17:53:03.000',16200,'2026-01-27 18:57:24.000','2026-01-27 18:57:24.000',21,NULL,NULL);
INSERT INTO `worklogs` VALUES (10054,10089,6,'Point avec le client','2026-01-06 08:28:30.000',12600,'2026-01-06 15:12:27.000','2026-01-06 15:12:27.000',6,NULL,NULL);
INSERT INTO `worklogs` VALUES (10055,10091,6,'Analyse','2026-01-11 12:09:00.000',5400,'2026-01-11 13:55:47.000','2026-01-11 13:55:47.000',6,NULL,NULL);
INSERT INTO `worklogs` VALUES (10056,10091,7,NULL,'2026-01-18 10:05:04.000',16200,'2026-01-18 11:25:52.000','2026-01-18 11:25:52.000',7,NULL,NULL);
INSERT INTO `worklogs` VALUES (10057,10094,5,'Développement','2026-01-25 15:06:42.000',24300,'2026-01-25 17:23:32.000','2026-01-25 17:23:32.000',5,NULL,NULL);
INSERT INTO `worklogs` VALUES (10058,10094,5,'Déploiement','2026-02-02 13:13:41.000',20700,'2026-02-02 17:08:24.000','2026-02-02 17:08:24.000',5,NULL,NULL);
INSERT INTO `worklogs` VALUES (10059,10094,5,NULL,'2026-02-11 15:06:41.000',14400,'2026-02-11 17:05:37.000','2026-02-11 17:05:37.000',5,NULL,NULL);
INSERT INTO `worklogs` VALUES (10060,10094,5,'Point avec le client','2026-02-21 12:38:46.000',9900,'2026-02-21 16:43:40.000','2026-02-21 16:43:40.000',5,NULL,NULL);
INSERT INTO `worklogs` VALUES (10061,10096,15,NULL,'2026-01-31 16:17:22.000',20700,'2026-01-31 18:45:15.000','2026-01-31 18:45:15.000',15,NULL,NULL);
INSERT INTO `worklogs` VALUES (10062,10096,15,NULL,'2026-02-10 18:48:53.000',27900,'2026-02-10 18:59:50.000','2026-02-10 18:59:50.000',15,NULL,NULL);
INSERT INTO `worklogs` VALUES (10063,10096,15,'Développement','2026-02-16 10:26:19.000',9900,'2026-02-16 17:08:46.000','2026-02-16 17:08:46.000',15,NULL,NULL);
INSERT INTO `worklogs` VALUES (10064,10096,15,'Revue de code','2026-02-16 18:59:41.000',15300,'2026-02-16 18:59:54.000','2026-02-16 18:59:54.000',15,NULL,NULL);
INSERT INTO `worklogs` VALUES (10065,10097,14,'Revue de code','2026-02-03 18:16:26.000',28800,'2026-02-03 18:21:58.000','2026-02-03 18:21:58.000',14,NULL,NULL);
INSERT INTO `worklogs` VALUES (10066,10097,14,'Revue de code','2026-02-09 16:40:49.000',7200,'2026-02-09 17:45:32.000','2026-02-09 17:45:32.000',14,NULL,NULL);
INSERT INTO `worklogs` VALUES (10067,10109,11,'Analyse','2026-02-13 15:42:12.000',5400,'2026-02-13 17:36:02.000','2026-02-13 17:36:02.000',11,NULL,NULL);
INSERT INTO `worklogs` VALUES (10068,10109,11,'Analyse','2026-02-20 17:10:01.000',5400,'2026-02-20 17:26:29.000','2026-02-20 17:26:29.000',11,NULL,NULL);
INSERT INTO `worklogs` VALUES (10069,10113,7,'Tests et recette','2026-02-14 17:42:22.000',19800,'2026-02-14 17:44:21.000','2026-02-14 17:44:21.000',7,NULL,NULL);
INSERT INTO `worklogs` VALUES (10070,10113,21,NULL,'2026-02-24 16:05:43.000',24300,'2026-02-24 17:41:24.000','2026-02-24 17:41:24.000',21,NULL,NULL);
INSERT INTO `worklogs` VALUES (10071,10123,15,'Point avec le client','2026-02-24 15:25:31.000',24300,'2026-02-24 17:12:15.000','2026-02-24 17:12:15.000',15,NULL,NULL);
INSERT INTO `worklogs` VALUES (10072,10124,15,'Tests et recette','2026-02-24 17:07:19.000',2700,'2026-02-24 18:05:17.000','2026-02-24 18:05:17.000',15,NULL,NULL);
INSERT INTO `worklogs` VALUES (10073,10124,15,'Revue de code','2026-02-26 10:42:45.000',19800,'2026-02-26 18:10:26.000','2026-02-26 18:10:26.000',15,NULL,NULL);
INSERT INTO `worklogs` VALUES (10074,10124,15,NULL,'2026-03-08 11:30:17.000',15300,'2026-03-08 12:38:56.000','2026-03-08 12:38:56.000',15,NULL,NULL);
INSERT INTO `worklogs` VALUES (10075,10126,4,'Analyse','2026-03-01 15:21:16.000',23400,'2026-03-01 18:19:22.000','2026-03-01 18:19:22.000',4,NULL,NULL);
INSERT INTO `worklogs` VALUES (10076,10126,4,'Tests et recette','2026-03-10 11:40:04.000',26100,'2026-03-10 15:30:27.000','2026-03-10 15:30:27.000',4,NULL,NULL);
INSERT INTO `worklogs` VALUES (10077,10126,4,'Déploiement','2026-03-20 17:51:10.000',18900,'2026-03-20 18:40:43.000','2026-03-20 18:40:43.000',4,NULL,NULL);
INSERT INTO `worklogs` VALUES (10078,10130,3,NULL,'2026-03-08 07:40:59.000',900,'2026-03-08 09:31:41.000','2026-03-08 09:31:41.000',3,NULL,NULL);
INSERT INTO `worklogs` VALUES (10079,10149,7,'Revue de code','2026-04-12 13:07:42.000',4500,'2026-04-12 13:59:00.000','2026-04-12 13:59:00.000',7,NULL,NULL);
INSERT INTO `worklogs` VALUES (10080,10149,7,'Développement','2026-04-15 14:02:33.000',18000,'2026-04-15 15:49:15.000','2026-04-15 15:49:15.000',7,NULL,NULL);
INSERT INTO `worklogs` VALUES (10081,10150,10,'Point avec le client','2026-04-11 15:27:09.000',25200,'2026-04-11 16:56:24.000','2026-04-11 16:56:24.000',10,NULL,NULL);
INSERT INTO `worklogs` VALUES (10082,10150,10,'Point avec le client','2026-04-21 11:29:26.000',10800,'2026-04-21 13:59:32.000','2026-04-21 13:59:32.000',10,NULL,NULL);
INSERT INTO `worklogs` VALUES (10083,10150,10,'Revue de code','2026-04-25 10:25:23.000',3600,'2026-04-25 13:31:49.000','2026-04-25 13:31:49.000',10,NULL,NULL);
INSERT INTO `worklogs` VALUES (10084,10158,3,'Développement','2026-04-22 16:17:55.000',9000,'2026-04-22 17:27:31.000','2026-04-22 17:27:31.000',3,NULL,NULL);
INSERT INTO `worklogs` VALUES (10085,10158,3,'Développement','2026-04-30 09:49:49.000',6300,'2026-04-30 16:32:43.000','2026-04-30 16:32:43.000',3,NULL,NULL);
INSERT INTO `worklogs` VALUES (10086,10162,22,'Point avec le client','2026-04-22 17:34:12.000',5400,'2026-04-22 17:48:05.000','2026-04-22 17:48:05.000',22,NULL,NULL);
INSERT INTO `worklogs` VALUES (10087,10162,23,'Développement','2026-04-28 11:47:01.000',6300,'2026-04-28 16:56:40.000','2026-04-28 16:56:40.000',23,NULL,NULL);
INSERT INTO `worklogs` VALUES (10088,10162,23,'Revue de code','2026-04-28 15:56:42.000',10800,'2026-04-28 17:07:35.000','2026-04-28 17:07:35.000',23,NULL,NULL);
INSERT INTO `worklogs` VALUES (10089,10163,9,'Point avec le client','2026-04-19 15:31:20.000',18900,'2026-04-19 17:21:35.000','2026-04-19 17:21:35.000',9,NULL,NULL);
INSERT INTO `worklogs` VALUES (10090,10163,3,NULL,'2026-04-25 11:46:39.000',19800,'2026-04-25 13:18:17.000','2026-04-25 13:18:17.000',3,NULL,NULL);
INSERT INTO `worklogs` VALUES (10091,10163,3,NULL,'2026-05-02 11:24:50.000',900,'2026-05-02 17:50:36.000','2026-05-02 17:50:36.000',3,NULL,NULL);
INSERT INTO `worklogs` VALUES (10092,10171,8,NULL,'2026-04-24 17:59:47.000',20700,'2026-04-24 17:59:52.000','2026-04-24 17:59:52.000',8,NULL,NULL);
INSERT INTO `worklogs` VALUES (10093,10171,8,NULL,'2026-04-26 13:50:25.000',11700,'2026-04-26 16:39:17.000','2026-04-26 16:39:17.000',8,NULL,NULL);
INSERT INTO `worklogs` VALUES (10094,10173,6,'Revue de code','2026-05-07 17:31:42.000',10800,'2026-05-07 17:58:52.000','2026-05-07 17:58:52.000',6,NULL,NULL);
INSERT INTO `worklogs` VALUES (10095,10173,6,'Point avec le client','2026-05-07 17:58:57.000',24300,'2026-05-07 17:59:44.000','2026-05-07 17:59:44.000',6,NULL,NULL);
INSERT INTO `worklogs` VALUES (10096,10174,2,'Analyse','2026-05-03 10:17:22.000',27900,'2026-05-03 10:49:22.000','2026-05-03 10:49:22.000',2,NULL,NULL);
INSERT INTO `worklogs` VALUES (10097,10174,2,'Analyse','2026-05-08 10:00:53.000',16200,'2026-05-08 15:00:39.000','2026-05-08 15:00:39.000',2,NULL,NULL);
INSERT INTO `worklogs` VALUES (10098,10178,2,NULL,'2026-05-20 07:35:41.000',12600,'2026-05-20 17:30:25.000','2026-05-20 17:30:25.000',2,NULL,NULL);
INSERT INTO `worklogs` VALUES (10099,10178,2,'Déploiement','2026-05-21 07:48:52.000',18900,'2026-05-21 17:34:24.000','2026-05-21 17:34:24.000',2,NULL,NULL);
INSERT INTO `worklogs` VALUES (10100,10178,2,NULL,'2026-05-21 12:11:15.000',19800,'2026-05-21 13:26:00.000','2026-05-21 13:26:00.000',2,NULL,NULL);
INSERT INTO `worklogs` VALUES (10101,10188,4,NULL,'2026-05-20 16:03:18.000',2700,'2026-05-20 17:58:52.000','2026-05-20 17:58:52.000',4,NULL,NULL);
INSERT INTO `worklogs` VALUES (10102,10198,3,'Analyse','2026-05-30 11:14:03.000',3600,'2026-05-30 12:52:20.000','2026-05-30 12:52:20.000',3,NULL,NULL);
INSERT INTO `worklogs` VALUES (10103,10198,3,'Point avec le client','2026-06-07 14:23:50.000',9000,'2026-06-07 16:46:17.000','2026-06-07 16:46:17.000',3,NULL,NULL);
INSERT INTO `worklogs` VALUES (10104,10201,9,'Développement','2026-05-29 06:51:19.000',20700,'2026-05-29 10:02:26.000','2026-05-29 10:02:26.000',9,NULL,NULL);
INSERT INTO `worklogs` VALUES (10105,10204,11,'Point avec le client','2026-06-06 09:52:45.000',12600,'2026-06-06 13:56:16.000','2026-06-06 13:56:16.000',11,NULL,NULL);
INSERT INTO `worklogs` VALUES (10106,10204,11,'Point avec le client','2026-06-12 09:26:47.000',15300,'2026-06-12 09:39:02.000','2026-06-12 09:39:02.000',11,NULL,NULL);
INSERT INTO `worklogs` VALUES (10107,10204,11,'Déploiement','2026-06-18 15:51:11.000',4500,'2026-06-18 17:12:56.000','2026-06-18 17:12:56.000',11,NULL,NULL);
INSERT INTO `worklogs` VALUES (10108,10208,17,'Tests et recette','2026-06-11 15:21:56.000',9900,'2026-06-11 17:03:58.000','2026-06-11 17:03:58.000',17,NULL,NULL);
INSERT INTO `worklogs` VALUES (10109,10214,2,'Revue de code','2026-06-16 11:34:58.000',20700,'2026-06-16 14:04:51.000','2026-06-16 14:04:51.000',2,NULL,NULL);
INSERT INTO `worklogs` VALUES (10110,10214,2,'Revue de code','2026-06-25 17:08:30.000',22500,'2026-06-25 17:30:00.000','2026-06-25 17:30:00.000',2,NULL,NULL);
INSERT INTO `worklogs` VALUES (10111,10214,2,'Point avec le client','2026-06-30 08:41:01.000',7200,'2026-06-30 10:56:12.000','2026-06-30 10:56:12.000',2,NULL,NULL);
INSERT INTO `worklogs` VALUES (10112,10217,17,'Revue de code','2026-06-19 11:56:09.000',6300,'2026-06-19 17:23:08.000','2026-06-19 17:23:08.000',17,NULL,NULL);
INSERT INTO `worklogs` VALUES (10113,10217,17,'Tests et recette','2026-06-27 10:07:33.000',27000,'2026-06-27 10:52:24.000','2026-06-27 10:52:24.000',17,NULL,NULL);
INSERT INTO `worklogs` VALUES (10114,10217,17,'Déploiement','2026-06-27 14:39:02.000',11700,'2026-06-27 16:23:47.000','2026-06-27 16:23:47.000',17,NULL,NULL);
INSERT INTO `worklogs` VALUES (10115,10225,8,NULL,'2026-06-28 11:52:22.000',16200,'2026-06-28 13:52:42.000','2026-06-28 13:52:42.000',8,NULL,NULL);
INSERT INTO `worklogs` VALUES (10116,10232,3,'Développement','2026-07-11 12:46:55.000',8100,'2026-07-11 17:50:03.000','2026-07-11 17:50:03.000',3,NULL,NULL);
INSERT INTO `worklogs` VALUES (10117,10232,3,'Revue de code','2026-07-21 06:00:24.000',18900,'2026-07-21 13:34:15.000','2026-07-21 13:34:15.000',3,NULL,NULL);
INSERT INTO `worklogs` VALUES (10118,10232,3,'Analyse','2026-07-31 13:20:20.000',6300,'2026-07-31 16:07:22.000','2026-07-31 16:07:22.000',3,NULL,NULL);
INSERT INTO `worklogs` VALUES (10119,10234,10,NULL,'2026-07-10 16:10:58.000',7200,'2026-07-10 16:35:56.000','2026-07-10 16:35:56.000',10,NULL,NULL);
INSERT INTO `worklogs` VALUES (10120,10234,10,'Revue de code','2026-07-10 17:29:12.000',21600,'2026-07-10 17:35:53.000','2026-07-10 17:35:53.000',10,NULL,NULL);
INSERT INTO `worklogs` VALUES (10121,10234,10,'Développement','2026-07-10 17:42:48.000',18900,'2026-07-10 17:45:20.000','2026-07-10 17:45:20.000',10,NULL,NULL);
INSERT INTO `worklogs` VALUES (10122,10234,10,'Tests et recette','2026-07-11 16:43:07.000',1800,'2026-07-11 16:56:43.000','2026-07-11 16:56:43.000',10,NULL,NULL);
INSERT INTO `worklogs` VALUES (10123,10239,16,'Tests et recette','2026-07-20 06:31:41.000',24300,'2026-07-20 08:44:01.000','2026-07-20 08:44:01.000',16,NULL,NULL);
INSERT INTO `worklogs` VALUES (10124,10239,16,'Analyse','2026-07-23 09:00:06.000',27000,'2026-07-23 17:16:40.000','2026-07-23 17:16:40.000',16,NULL,NULL);
INSERT INTO `worklogs` VALUES (10125,10241,5,NULL,'2026-07-20 10:54:45.000',1800,'2026-07-20 15:37:49.000','2026-07-20 15:37:49.000',5,NULL,NULL);
INSERT INTO `worklogs` VALUES (10126,10241,5,'Analyse','2026-07-20 15:16:37.000',22500,'2026-07-20 16:28:22.000','2026-07-20 16:28:22.000',5,NULL,NULL);
INSERT INTO `worklogs` VALUES (10127,10248,21,'Déploiement','2026-08-02 13:01:24.000',15300,'2026-08-02 13:31:33.000','2026-08-02 13:31:33.000',21,NULL,NULL);
INSERT INTO `worklogs` VALUES (10128,10248,21,'Point avec le client','2026-08-08 11:40:31.000',4500,'2026-08-08 16:54:36.000','2026-08-08 16:54:36.000',21,NULL,NULL);
INSERT INTO `worklogs` VALUES (10129,10249,2,NULL,'2026-07-30 09:23:33.000',5400,'2026-07-30 15:17:07.000','2026-07-30 15:17:07.000',2,NULL,NULL);
INSERT INTO `worklogs` VALUES (10130,10249,2,'Déploiement','2026-08-01 06:50:38.000',7200,'2026-08-01 09:57:15.000','2026-08-01 09:57:15.000',2,NULL,NULL);
INSERT INTO `worklogs` VALUES (10131,10249,2,'Point avec le client','2026-08-06 16:17:20.000',18900,'2026-08-06 17:00:30.000','2026-08-06 17:00:30.000',2,NULL,NULL);
INSERT INTO `worklogs` VALUES (10132,10250,23,'Développement','2026-07-31 09:18:29.000',27900,'2026-07-31 11:42:38.000','2026-07-31 11:42:38.000',23,NULL,NULL);
INSERT INTO `worklogs` VALUES (10133,10250,23,'Analyse','2026-08-05 16:31:59.000',10800,'2026-08-05 17:25:15.000','2026-08-05 17:25:15.000',23,NULL,NULL);
INSERT INTO `worklogs` VALUES (10134,10254,16,NULL,'2026-08-05 06:29:16.000',22500,'2026-08-05 08:41:49.000','2026-08-05 08:41:49.000',16,NULL,NULL);
INSERT INTO `worklogs` VALUES (10135,10254,16,'Développement','2026-08-09 16:34:13.000',24300,'2026-08-09 17:27:49.000','2026-08-09 17:27:49.000',16,NULL,NULL);
INSERT INTO `worklogs` VALUES (10136,10254,16,NULL,'2026-08-16 15:55:22.000',9000,'2026-08-16 17:29:29.000','2026-08-16 17:29:29.000',16,NULL,NULL);
INSERT INTO `worklogs` VALUES (10137,10257,13,'Développement','2026-08-12 08:41:48.000',17100,'2026-08-12 16:56:43.000','2026-08-12 16:56:43.000',13,NULL,NULL);
INSERT INTO `worklogs` VALUES (10138,10260,22,'Tests et recette','2026-08-14 06:34:47.000',18000,'2026-08-14 12:03:57.000','2026-08-14 12:03:57.000',22,NULL,NULL);
INSERT INTO `worklogs` VALUES (10139,10265,17,'Analyse','2026-08-26 17:57:08.000',5400,'2026-08-26 17:59:05.000','2026-08-26 17:59:05.000',17,NULL,NULL);
INSERT INTO `worklogs` VALUES (10140,10265,17,NULL,'2026-09-02 10:07:04.000',1800,'2026-09-02 17:18:47.000','2026-09-02 17:18:47.000',17,NULL,NULL);
INSERT INTO `worklogs` VALUES (10141,10265,17,'Développement','2026-09-02 13:51:35.000',7200,'2026-09-02 14:27:21.000','2026-09-02 14:27:21.000',17,NULL,NULL);
INSERT INTO `worklogs` VALUES (10142,10265,17,NULL,'2026-09-02 17:29:52.000',26100,'2026-09-02 17:38:50.000','2026-09-02 17:38:50.000',17,NULL,NULL);
INSERT INTO `worklogs` VALUES (10143,10275,23,'Point avec le client','2026-09-08 07:53:34.000',21600,'2026-09-08 13:33:23.000','2026-09-08 13:33:23.000',23,NULL,NULL);
INSERT INTO `worklogs` VALUES (10144,10275,23,NULL,'2026-09-13 10:05:16.000',23400,'2026-09-13 16:06:56.000','2026-09-13 16:06:56.000',23,NULL,NULL);
INSERT INTO `worklogs` VALUES (10145,10275,23,NULL,'2026-09-20 12:05:44.000',4500,'2026-09-20 17:17:12.000','2026-09-20 17:17:12.000',23,NULL,NULL);
INSERT INTO `worklogs` VALUES (10146,10287,7,'Déploiement','2026-09-19 09:23:54.000',25200,'2026-09-19 12:58:44.000','2026-09-19 12:58:44.000',7,NULL,NULL);
INSERT INTO `worklogs` VALUES (10147,10287,11,'Déploiement','2026-09-23 17:22:17.000',6300,'2026-09-23 17:59:01.000','2026-09-23 17:59:01.000',11,NULL,NULL);
INSERT INTO `worklogs` VALUES (10148,10287,11,'Développement','2026-09-28 07:01:02.000',13500,'2026-09-28 17:04:25.000','2026-09-28 17:04:25.000',11,NULL,NULL);
INSERT INTO `worklogs` VALUES (10149,10287,9,'Revue de code','2026-09-28 15:58:59.000',27000,'2026-09-28 17:15:06.000','2026-09-28 17:15:06.000',9,NULL,NULL);
INSERT INTO `worklogs` VALUES (10150,10290,17,'Tests et recette','2026-09-25 13:01:20.000',4500,'2026-09-25 13:46:57.000','2026-09-25 13:46:57.000',17,NULL,NULL);
INSERT INTO `worklogs` VALUES (10151,10290,17,NULL,'2026-09-27 13:10:11.000',6300,'2026-09-27 15:08:06.000','2026-09-27 15:08:06.000',17,NULL,NULL);
INSERT INTO `worklogs` VALUES (10152,10290,17,NULL,'2026-09-27 15:51:40.000',4500,'2026-09-27 17:32:35.000','2026-09-27 17:32:35.000',17,NULL,NULL);
INSERT INTO `worklogs` VALUES (10153,10293,21,'Déploiement','2026-09-27 14:00:32.000',23400,'2026-09-27 17:14:45.000','2026-09-27 17:14:45.000',21,NULL,NULL);
INSERT INTO `worklogs` VALUES (10154,10293,21,NULL,'2026-09-29 07:37:52.000',27900,'2026-09-29 10:40:49.000','2026-09-29 10:40:49.000',21,NULL,NULL);
INSERT INTO `worklogs` VALUES (10155,10296,22,'Revue de code','2026-09-29 11:00:50.000',21600,'2026-09-29 13:27:08.000','2026-09-29 13:27:08.000',22,NULL,NULL);
INSERT INTO `worklogs` VALUES (10156,10297,5,NULL,'2026-09-28 13:49:24.000',9900,'2026-09-28 15:09:31.000','2026-09-28 15:09:31.000',5,NULL,NULL);
INSERT INTO `worklogs` VALUES (10157,10299,8,'Développement','2026-10-01 16:00:00.000',26100,'2026-10-01 16:00:01.000','2026-10-01 16:00:01.000',8,NULL,NULL);
/*!40000 ALTER TABLE `worklogs` ENABLE KEYS */;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*M!100616 SET NOTE_VERBOSITY=@OLD_NOTE_VERBOSITY */;

