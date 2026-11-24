<?php
/**
 * Plugin Name: Demo
 * Version: 1.2.3
 */
defined( 'ABSPATH' ) || exit;

add_action( 'wp_ajax_nopriv_demo_search', 'demo_search' );
add_action( 'wp_ajax_demo_save', 'demo_save' );
add_action( 'admin_post_demo_export', 'demo_export' );

add_action( 'rest_api_init', function () {
	register_rest_route( 'demo/v1', '/items/(?P<id>\d+)', array(
		'methods'             => WP_REST_Server::READABLE,
		'callback'            => 'demo_item',
		'permission_callback' => '__return_true',
	) );
	register_rest_route( 'demo/v1', '/items', array(
		'methods'  => 'POST',
		'callback' => 'demo_create',
	) );
} );

function demo_search() {
	global $wpdb;
	$id    = absint( $_GET['id'] );
	$q     = sanitize_text_field( wp_unslash( $_POST['q'] ) );
	$email = sanitize_email( $_POST['email'] );
	if ( ! in_array( $_GET['order'], array( 'asc', 'desc' ), true ) ) { return; }
	$rows = $wpdb->get_results( "SELECT * FROM t WHERE name = '" . $_GET['raw'] . "'" );
	$safe = $wpdb->get_results( $wpdb->prepare( "SELECT * FROM t WHERE id = %d", $id ) );
}
