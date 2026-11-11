import 'package:flutter/material.dart';
import 'api_client.dart';

class DettaglioSpedizioneScreen extends StatefulWidget {
  final Map<String, dynamic> spedizione;

  const DettaglioSpedizioneScreen({super.key, required this.spedizione});

  @override
  State<DettaglioSpedizioneScreen> createState() => _DettaglioSpedizioneScreenState();
}

class _DettaglioSpedizioneScreenState extends State<DettaglioSpedizioneScreen> {
  final TextEditingController _notaController = TextEditingController();
  late String _statoAttuale;
  bool _isSaving = false;

  @override
  void initState() {
    super.initState();
    _statoAttuale = widget.spedizione['stato_corrente'] ?? widget.spedizione['stato'] ?? 'In attesa';
  }

  @override
  void dispose() {
    _notaController.dispose();
    super.dispose();
  }

  Future<void> _cambiaStato(String nuovoStato) async {
    setState(() => _isSaving = true);
    try {
      final id = widget.spedizione['id'].toString();
      await apiClient.aggiornaStato(id, nuovoStato);
      setState(() {
        _statoAttuale = nuovoStato;
      });
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Stato aggiornato a: $nuovoStato')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Errore durante l\'aggiornamento: $e'), backgroundColor: Colors.red),
        );
      }
    } finally {
      if (mounted) setState(() => _isSaving = false);
    }
  }

  Future<void> _aggiungiNota() async {
    if (_notaController.text.trim().isEmpty) return;

    setState(() => _isSaving = true);
    try {
      final id = widget.spedizione['id'].toString();
      await apiClient.aggiungiNota(id, _notaController.text.trim());
      _notaController.clear();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Nota aggiunta con successo')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Errore salvataggio nota: $e'), backgroundColor: Colors.red),
        );
      }
    } finally {
      if (mounted) setState(() => _isSaving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final id = widget.spedizione['id']?.toString() ?? 'N/D';
    final indirizzo = widget.spedizione['indirizzo_destinazione'] ?? widget.spedizione['indirizzo'] ?? 'Indirizzo non presente';

    return Scaffold(
      appBar: AppBar(
        title: Text('Spedizione #$id'),
      ),
      body: _isSaving
          ? const Center(child: CircularProgressIndicator())
          : SingleChildScrollView(
        padding: const EdgeInsets.all(16.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Card(
              child: Padding(
                padding: const EdgeInsets.all(16.0),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('Indirizzo: $indirizzo', style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
                    const SizedBox(height: 8),
                    Text('Stato Attuale: $_statoAttuale', style: const TextStyle(fontSize: 16, color: Colors.deepOrange)),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 20),
            const Text('Aggiorna Stato', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
            const SizedBox(height: 8),
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceAround,
              children: [
                ElevatedButton(
                  onPressed: () => _cambiaStato('In Consegna'),
                  child: const Text('In Consegna'),
                ),
                ElevatedButton(
                  style: ElevatedButton.styleFrom(backgroundColor: Colors.green, foregroundColor: Colors.white),
                  onPressed: () => _cambiaStato('Consegnato'),
                  child: const Text('Consegnato'),
                ),
              ],
            ),
            const SizedBox(height: 24),
            const Text('Aggiungi Nota', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
            const SizedBox(height: 8),
            TextField(
              controller: _notaController,
              decoration: const InputDecoration(
                border: OutlineInputBorder(),
                hintText: 'Inserisci una nota di consegna...',
              ),
              maxLines: 3,
            ),
            const SizedBox(height: 8),
            ElevatedButton.icon(
              onPressed: _aggiungiNota,
              icon: const Icon(Icons.send),
              label: const Text('Salva Nota'),
            ),
          ],
        ),
      ),
    );
  }
}