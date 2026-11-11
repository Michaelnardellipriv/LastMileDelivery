import 'package:dio/dio.dart';

class ApiClient {
  final Dio dio = Dio(
    BaseOptions(
      baseUrl: 'http://localhost:8080',
      headers: {
        'Content-Type': 'application/json',
        'X-Id-Operatore': '1',
      },
    ),
  );

  /// Recupera l'elenco delle spedizioni dal backend
  Future<List<dynamic>> getSpedizioni() async {
    final response = await dio.get('/api/spedizioni');

    if (response.data is Map<String, dynamic>) {
      final mapData = response.data as Map<String, dynamic>;
      if (mapData.containsKey('dati')) {
        return mapData['dati'] as List<dynamic>;
      }
    } else if (response.data is List) {
      return response.data as List<dynamic>;
    }

    return [];
  }

  /// Cambia lo stato di una spedizione (PATCH /api/spedizioni/{id}/stato)
  Future<void> aggiornaStato(dynamic id, String nuovoStato) async {
    final intId = id is int ? id : int.parse(id.toString());
    await dio.patch(
      '/api/spedizioni/$intId/stato',
      data: {
        'stato': nuovoStato,
      },
    );
  }

  /// Aggiunge una nota interna a una spedizione (POST /api/spedizioni/{id}/note)
  Future<void> aggiungiNota(dynamic id, String testoNota) async {
    final intId = id is int ? id : int.parse(id.toString());
    await dio.post(
      '/api/spedizioni/$intId/note',
      data: {
        'nota': testoNota,
      },
    );
  }
}

final apiClient = ApiClient();