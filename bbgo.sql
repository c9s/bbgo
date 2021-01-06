-- MySQL dump 10.13  Distrib 5.7.29, for osx10.15 (x86_64)
--
-- Host: localhost    Database: bbgo
-- ------------------------------------------------------
-- Server version	5.7.29

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Table structure for table `binance_klines`
--

DROP TABLE IF EXISTS `binance_klines`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `binance_klines` (
  `gid` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `exchange` varchar(10) NOT NULL,
  `start_time` datetime(3) NOT NULL,
  `end_time` datetime(3) NOT NULL,
  `interval` varchar(3) NOT NULL,
  `symbol` varchar(7) NOT NULL,
  `open` decimal(16,8) unsigned NOT NULL,
  `high` decimal(16,8) unsigned NOT NULL,
  `low` decimal(16,8) unsigned NOT NULL,
  `close` decimal(16,8) unsigned NOT NULL DEFAULT '0.00000000',
  `volume` decimal(16,8) unsigned NOT NULL DEFAULT '0.00000000',
  `closed` tinyint(1) NOT NULL DEFAULT '1',
  `last_trade_id` int(10) unsigned NOT NULL DEFAULT '0',
  `num_trades` int(10) unsigned NOT NULL DEFAULT '0',
  PRIMARY KEY (`gid`),
  KEY `klines_end_time_symbol_interval` (`end_time`,`symbol`,`interval`)
) ENGINE=InnoDB AUTO_INCREMENT=1565318 DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `goose_db_version`
--

DROP TABLE IF EXISTS `goose_db_version`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `goose_db_version` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `version_id` bigint(20) NOT NULL,
  `is_applied` tinyint(1) NOT NULL,
  `tstamp` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `id` (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=92 DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `klines`
--

DROP TABLE IF EXISTS `klines`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `klines` (
  `gid` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `exchange` varchar(10) NOT NULL,
  `start_time` datetime(3) NOT NULL,
  `end_time` datetime(3) NOT NULL,
  `interval` varchar(3) NOT NULL,
  `symbol` varchar(7) NOT NULL,
  `open` decimal(16,8) unsigned NOT NULL,
  `high` decimal(16,8) unsigned NOT NULL,
  `low` decimal(16,8) unsigned NOT NULL,
  `close` decimal(16,8) unsigned NOT NULL DEFAULT '0.00000000',
  `volume` decimal(16,8) unsigned NOT NULL DEFAULT '0.00000000',
  `closed` tinyint(1) NOT NULL DEFAULT '1',
  `last_trade_id` int(10) unsigned NOT NULL DEFAULT '0',
  `num_trades` int(10) unsigned NOT NULL DEFAULT '0',
  PRIMARY KEY (`gid`),
  KEY `klines_end_time_symbol_interval` (`end_time`,`symbol`,`interval`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `max_klines`
--

DROP TABLE IF EXISTS `max_klines`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `max_klines` (
  `gid` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `exchange` varchar(10) NOT NULL,
  `start_time` datetime(3) NOT NULL,
  `end_time` datetime(3) NOT NULL,
  `interval` varchar(3) NOT NULL,
  `symbol` varchar(7) NOT NULL,
  `open` decimal(16,8) unsigned NOT NULL,
  `high` decimal(16,8) unsigned NOT NULL,
  `low` decimal(16,8) unsigned NOT NULL,
  `close` decimal(16,8) unsigned NOT NULL DEFAULT '0.00000000',
  `volume` decimal(16,8) unsigned NOT NULL DEFAULT '0.00000000',
  `closed` tinyint(1) NOT NULL DEFAULT '1',
  `last_trade_id` int(10) unsigned NOT NULL DEFAULT '0',
  `num_trades` int(10) unsigned NOT NULL DEFAULT '0',
  PRIMARY KEY (`gid`),
  KEY `klines_end_time_symbol_interval` (`end_time`,`symbol`,`interval`)
) ENGINE=InnoDB AUTO_INCREMENT=772024 DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `okex_klines`
--

DROP TABLE IF EXISTS `okex_klines`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `okex_klines` (
  `gid` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `exchange` varchar(10) NOT NULL,
  `start_time` datetime(3) NOT NULL,
  `end_time` datetime(3) NOT NULL,
  `interval` varchar(3) NOT NULL,
  `symbol` varchar(7) NOT NULL,
  `open` decimal(16,8) unsigned NOT NULL,
  `high` decimal(16,8) unsigned NOT NULL,
  `low` decimal(16,8) unsigned NOT NULL,
  `close` decimal(16,8) unsigned NOT NULL DEFAULT '0.00000000',
  `volume` decimal(16,8) unsigned NOT NULL DEFAULT '0.00000000',
  `closed` tinyint(1) NOT NULL DEFAULT '1',
  `last_trade_id` int(10) unsigned NOT NULL DEFAULT '0',
  `num_trades` int(10) unsigned NOT NULL DEFAULT '0',
  PRIMARY KEY (`gid`),
  KEY `klines_end_time_symbol_interval` (`end_time`,`symbol`,`interval`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `orders`
--

DROP TABLE IF EXISTS `orders`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `orders` (
  `gid` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `exchange` varchar(24) NOT NULL DEFAULT '',
  `order_id` bigint(20) unsigned NOT NULL,
  `client_order_id` varchar(42) NOT NULL DEFAULT '',
  `order_type` varchar(16) NOT NULL,
  `symbol` varchar(9) DEFAULT NULL,
  `status` varchar(12) NOT NULL,
  `time_in_force` varchar(4) NOT NULL,
  `price` decimal(16,8) unsigned NOT NULL,
  `stop_price` decimal(16,8) unsigned NOT NULL,
  `quantity` decimal(16,8) unsigned NOT NULL,
  `executed_quantity` decimal(16,8) unsigned NOT NULL DEFAULT '0.00000000',
  `side` varchar(4) NOT NULL DEFAULT '',
  `is_working` tinyint(1) NOT NULL DEFAULT '0',
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`gid`),
  UNIQUE KEY `orders_order_id` (`order_id`,`exchange`),
  KEY `orders_symbol` (`exchange`,`symbol`)
) ENGINE=InnoDB AUTO_INCREMENT=2903 DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `trades`
--

DROP TABLE IF EXISTS `trades`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `trades` (
  `gid` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `id` bigint(20) unsigned DEFAULT NULL,
  `exchange` varchar(24) NOT NULL DEFAULT '',
  `symbol` varchar(9) DEFAULT NULL,
  `price` decimal(16,8) unsigned NOT NULL,
  `quantity` decimal(16,8) unsigned NOT NULL,
  `quote_quantity` decimal(16,8) unsigned NOT NULL,
  `fee` decimal(16,8) unsigned NOT NULL,
  `fee_currency` varchar(4) NOT NULL,
  `is_buyer` tinyint(1) NOT NULL DEFAULT '0',
  `is_maker` tinyint(1) NOT NULL DEFAULT '0',
  `side` varchar(4) NOT NULL DEFAULT '',
  `traded_at` datetime(3) NOT NULL,
  `order_id` bigint(20) unsigned NOT NULL,
  PRIMARY KEY (`gid`),
  UNIQUE KEY `id` (`id`),
  KEY `trades_symbol` (`exchange`,`symbol`),
  KEY `trades_symbol_fee_currency` (`exchange`,`symbol`,`fee_currency`,`traded_at`),
  KEY `trades_traded_at_symbol` (`exchange`,`traded_at`,`symbol`)
) ENGINE=InnoDB AUTO_INCREMENT=6621 DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2021-01-06 15:13:53
